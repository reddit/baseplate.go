package tracing

import (
	"context"
	"math/rand"
	"time"

	"github.com/reddit/baseplate.go/timebp"
)

// SpanType enum.
type SpanType int

// SpanType values.
const (
	SpanTypeLocal SpanType = iota
	SpanTypeClient
	SpanTypeServer
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

// Span defines a tracing span.
type Span struct {
	tracer *Tracer

	name     string
	traceID  uint64
	spanID   uint64
	parentID uint64
	sampled  bool

	component string

	start    time.Time
	end      time.Time
	spanType SpanType
	flags    int64

	counters map[string]float64
	tags     map[string]interface{}
}

func newSpan(tracer *Tracer, spanType SpanType) *Span {
	if tracer == nil {
		tracer = &GlobalTracer
	}
	return &Span{
		tracer:   tracer,
		traceID:  rand.Uint64(),
		spanID:   rand.Uint64(),
		start:    time.Now(),
		spanType: spanType,

		counters: make(map[string]float64),
		tags: map[string]interface{}{
			ZipkinBinaryAnnotationKeyComponent: baseplateComponent,
		},
	}
}

// ToZipkinSpan returns a ZipkinSpan with data copied from this span.
//
// If it's called before End is called,
// Duration and some annotations might be incorrect/incomplete.
func (s *Span) ToZipkinSpan() ZipkinSpan {
	zs := ZipkinSpan{
		TraceID:  s.traceID,
		Name:     s.name,
		SpanID:   s.spanID,
		Start:    timebp.TimestampMicrosecond(s.start),
		ParentID: s.parentID,
	}
	end := s.end
	if end.IsZero() {
		end = time.Now()
	}
	zs.Duration = timebp.DurationMicrosecond(end.Sub(s.start))

	var endpoint ZipkinEndpointInfo
	if s.tracer != nil {
		endpoint = s.tracer.Endpoint
	}

	switch s.spanType {
	case SpanTypeServer:
		zs.TimeAnnotations = []ZipkinTimeAnnotation{
			{
				Endpoint:  endpoint,
				Key:       ZipkinTimeAnnotationKeyServerReceive,
				Timestamp: timebp.TimestampMicrosecond(s.start),
			},
			{
				Endpoint:  endpoint,
				Key:       ZipkinTimeAnnotationKeyServerSend,
				Timestamp: timebp.TimestampMicrosecond(end),
			},
		}
	case SpanTypeClient:
		zs.TimeAnnotations = []ZipkinTimeAnnotation{
			{
				Endpoint:  endpoint,
				Key:       ZipkinTimeAnnotationKeyClientSend,
				Timestamp: timebp.TimestampMicrosecond(s.start),
			},
			{
				Endpoint:  endpoint,
				Key:       ZipkinTimeAnnotationKeyClientReceive,
				Timestamp: timebp.TimestampMicrosecond(end),
			},
		}
	}

	zs.BinaryAnnotations = make([]ZipkinBinaryAnnotation, 0, len(s.counters)+len(s.tags))
	for key, value := range s.counters {
		zs.BinaryAnnotations = append(
			zs.BinaryAnnotations,
			ZipkinBinaryAnnotation{
				Endpoint: endpoint,
				Key:      counterKeyPrefix + key,
				Value:    value,
			},
		)
	}
	for key, value := range s.tags {
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

func (s *Span) isDebugSet() bool {
	return s.flags&FlagMaskDebug != 0
}

// ShouldSample returns true if this span should be sampled.
//
// If the span's debug flag was set, then this function will always return true.
func (s *Span) ShouldSample() bool {
	return s.sampled || s.isDebugSet()
}

// End ends a span.
//
// If err is non-nil, the error tag will also be set to true.
//
// It also sends the span into tracer, if the span should be sampled.
// The context object passed in is used to control the timeout of the sending,
// see Tracer.MaxRecordTimeout for more info.
func (s *Span) End(ctx context.Context, err error) error {
	s.end = time.Now()

	if err != nil {
		s.SetTag(ZipkinBinaryAnnotationKeyError, true)
	}
	if s.isDebugSet() {
		s.SetTag(ZipkinBinaryAnnotationKeyDebug, true)
	}
	if s.spanType == SpanTypeLocal && s.component != "" {
		s.SetTag(ZipkinBinaryAnnotationKeyLocalComponent, s.component)
	}

	if s.tracer != nil {
		return s.tracer.Record(ctx, s)
	}
	return nil
}

// AddCounter adds delta to a counter annotation.
func (s *Span) AddCounter(key string, delta float64) {
	s.counters[key] += delta
}

// SetTag sets a binary tag annotation.
func (s *Span) SetTag(key string, value interface{}) {
	s.tags[key] = value
}

// SetDebug sets or unsets the debug flag of this span.
func (s *Span) SetDebug(v bool) {
	if v {
		s.flags |= FlagMaskDebug
	} else {
		s.flags &= ^FlagMaskDebug
	}
}

// CreateLocalChild creates a local child-span with given name and component.
//
// component is optional local component name and could be empty string.
//
// Timestamps, counters, and tags won't be inherited.
// Parent id will be inherited from the span id,
// and span id will be randomly generated.
// Trace id, sampled, and flags will be copied over.
func (s *Span) CreateLocalChild(name string, component string) *Span {
	return s.createChild(name, SpanTypeLocal, component)
}

// CreateClientChild creates a client child-span with given name.
//
// A client child-span should be used to make requests to other upstream
// servers.
//
// Timestamps, counters, and tags won't be inherited.
// Parent id will be inherited from the span id,
// and span id will be randomly generated.
// Trace id, sampled, and flags will be copied over.
func (s *Span) CreateClientChild(name string) *Span {
	return s.createChild(name, SpanTypeClient, "")
}

func (s *Span) createChild(name string, spanType SpanType, component string) *Span {
	span := newSpan(s.tracer, spanType)
	span.name = name
	span.spanType = spanType
	span.component = component

	span.parentID = s.spanID
	span.traceID = s.traceID
	span.sampled = s.sampled
	span.flags = s.flags

	return span
}

// ChildAndThriftContext creates both a client child span and a context can be
// used by the thrift client code.
//
// A thrift client call would look like:
//
//     clientCtx, span := parentSpan.ChildAndThriftContext(ctx, "myCall")
//     result, err := client.MyCall(clientCtx, arg1, arg2)
//     span.End(ctx, err)
func (s *Span) ChildAndThriftContext(ctx context.Context, name string) (context.Context, *Span) {
	span := s.CreateClientChild(name)
	ctx = CreateThriftContextFromSpan(ctx, span)
	return ctx, span
}
