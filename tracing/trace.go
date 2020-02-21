package tracing

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/reddit/baseplate.go/timebp"
)

// FlagMask values.
//
// Reference: https://github.com/reddit/baseplate.py/blob/1ca8488bcd42c8786e6a3db35b2a99517fd07a99/baseplate/observers/tracing.py#L60-L64
const (
	// When set, this trace passes all samplers.
	FlagMaskDebug int64 = 1
)

const (
	counterKeyPrefix   = "counter."
	baseplateComponent = "baseplate"
)

type trace struct {
	tracer *Tracer

	name     string
	traceID  uint64
	spanID   uint64
	parentID uint64
	sampled  bool
	flags    int64

	timeAnnotationReceiveKey string
	timeAnnotationSendKey    string
	start                    time.Time
	stop                     time.Time

	counters map[string]float64
	tags     map[string]string
}

func newTrace(tracer *Tracer, name string) *trace {
	if tracer == nil {
		tracer = &globalTracer
	}
	return &trace{
		tracer: tracer,

		name:    name,
		traceID: rand.Uint64(),
		spanID:  rand.Uint64(),
		start:   time.Now(),

		counters: make(map[string]float64),
		tags: map[string]string{
			ZipkinBinaryAnnotationKeyComponent: baseplateComponent,
		},
	}
}

func (t *trace) addCounter(key string, delta float64) {
	t.counters[key] += delta
}

func (t *trace) setTag(key string, value interface{}) {
	t.tags[key] = fmt.Sprintf("%v", value)
}

func (t *trace) toZipkinSpan() ZipkinSpan {
	zs := ZipkinSpan{
		TraceID:  t.traceID,
		Name:     t.name,
		SpanID:   t.spanID,
		Start:    timebp.TimestampMicrosecond(t.start),
		ParentID: t.parentID,
	}
	end := t.stop
	if end.IsZero() {
		end = time.Now()
	}
	zs.Duration = timebp.DurationMicrosecond(end.Sub(t.start))

	var endpoint ZipkinEndpointInfo
	if t.tracer != nil {
		endpoint = t.tracer.endpoint
	}

	if t.timeAnnotationReceiveKey != "" {
		zs.TimeAnnotations = append(zs.TimeAnnotations, ZipkinTimeAnnotation{
			Endpoint:  endpoint,
			Key:       t.timeAnnotationReceiveKey,
			Timestamp: timebp.TimestampMicrosecond(t.start),
		})
	}

	if t.timeAnnotationSendKey != "" {
		zs.TimeAnnotations = append(zs.TimeAnnotations, ZipkinTimeAnnotation{
			Endpoint:  endpoint,
			Key:       t.timeAnnotationSendKey,
			Timestamp: timebp.TimestampMicrosecond(end),
		})
	}

	zs.BinaryAnnotations = make([]ZipkinBinaryAnnotation, 0, len(t.counters)+len(t.tags))
	for key, value := range t.counters {
		zs.BinaryAnnotations = append(
			zs.BinaryAnnotations,
			ZipkinBinaryAnnotation{
				Endpoint: endpoint,
				Key:      counterKeyPrefix + key,
				Value:    value,
			},
		)
	}
	for key, value := range t.tags {
		zs.BinaryAnnotations = append(
			zs.BinaryAnnotations,
			ZipkinBinaryAnnotation{
				Endpoint: endpoint,
				Key:      key,
				Value:    value,
			},
		)
	}

	return zs
}

func (t *trace) isDebugSet() bool {
	return t.flags&FlagMaskDebug != 0
}

func (t *trace) setDebug(v bool) {
	if v {
		t.flags |= FlagMaskDebug
	} else {
		t.flags &= ^FlagMaskDebug
	}
}

// shouldSample returns true if this span should be sampled.
//
// If the span's debug flag was set, then this function will always return true.
func (t *trace) shouldSample() bool {
	return t.sampled || t.isDebugSet()
}

func (t *trace) publish(ctx context.Context) error {
	if !t.shouldSample() || t.tracer == nil {
		return nil
	}
	return t.tracer.Record(ctx, t.toZipkinSpan())
}
