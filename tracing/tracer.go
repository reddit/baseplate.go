package tracing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/mqsend"
	"github.com/reddit/baseplate.go/randbp"
	"github.com/reddit/baseplate.go/runtimebp"

	opentracing "github.com/opentracing/opentracing-go"
)

var (
	_ opentracing.Tracer = (*Tracer)(nil)
)

// Configuration values for the message queue.
const (
	// Max size of serialized span in bytes.
	MaxSpanSize = 102400
	// Max number of spans allowed in the message queue at one time.
	MaxQueueSize = 10000
	// Prefix added to the queue name.
	QueueNamePrefix = "traces-"
	// The default MaxRecordTimeout used in Tracers.
	DefaultMaxRecordTimeout = time.Millisecond * 50
)

// Register our GlobalTracer as opentracing's global tracer.
func init() {
	opentracing.SetGlobalTracer(&GlobalTracer)
}

// GlobalTracer is the default Tracer to be used.
var GlobalTracer Tracer

// A Tracer creates and manages spans.
type Tracer struct {
	// SampleRate should be in the range of [0, 1].
	// When SampleRate >= 1, all traces will be recoreded;
	// When SampleRate <= 0, none of the traces will be recorded,
	// except the ones with debug flag set.
	SampleRate float64

	// Recorder sends the sampled spans.
	//
	// If Recorder is nil, no spans will be sampled,
	// including the ones with debug flag set.
	Recorder mqsend.MessageQueue

	// Logger, if non-nil, will be used to log additional informations Record
	// returned certain errors.
	Logger log.Wrapper

	// The endpoint info to be reported to zipkin with annotations.
	Endpoint ZipkinEndpointInfo

	// The max timeout applied to Record function.
	//
	// If the passed in context object has an earlier deadline set,
	// that deadline will be respected instead.
	//
	// If MaxRecordTimeout <= 0, DefaultMaxRecordTimeout will be used.
	MaxRecordTimeout time.Duration
}

// InitGlobalTracer initializes GlobalTracer.
//
// queueName should not contain "traces-" prefix, it will be auto added.
//
// This function will try to get the first local IPv4 address of this machine
// and set the GlobalTracer.Endpoint.
// If it fails to do so, UndefinedIP will be used instead,
// and the error will be logged if logger is non-nil.
func InitGlobalTracer(
	serviceName string,
	queueName string,
	sampleRate float64,
	logger log.Wrapper,
) error {
	recorder, err := mqsend.OpenMessageQueue(mqsend.MessageQueueConfig{
		Name:           QueueNamePrefix + queueName,
		MaxQueueSize:   MaxQueueSize,
		MaxMessageSize: MaxSpanSize,
	})
	if err != nil {
		return err
	}
	GlobalTracer.Recorder = recorder

	ip, err := runtimebp.GetFirstIPv4()
	if err != nil && logger != nil {
		logger(`Unable to get local ip address: ` + err.Error())
	}
	GlobalTracer.Endpoint = ZipkinEndpointInfo{
		ServiceName: serviceName,
		IPv4:        ip,
	}

	GlobalTracer.SampleRate = sampleRate
	GlobalTracer.Logger = logger
	opentracing.SetGlobalTracer(&GlobalTracer)
	return nil
}

// Close closes the tracer's Recorder.
//
// After Close is called,
// Recorder will be set to nil and no more spans will be sampled.
func (t *Tracer) Close() error {
	if t.Recorder == nil {
		return nil
	}
	err := t.Recorder.Close()
	t.Recorder = nil
	return err
}

// NewTrace creates a new trace (top level local span).
func (t *Tracer) NewTrace(name string) *Span {
	span := newSpan(t, name, SpanTypeLocal)
	span.trace.traceID = rand.Uint64()
	span.trace.sampled = randbp.ShouldSampleWithRate(t.SampleRate)
	span.onStart()
	return span
}

// Record records a span with the Recorder.
//
// Span.Stop(), Span.Finish(), and Span.FinishWithOptions() call this function
// automatically.
// In most cases that should be enough and you should not call this function
// directly.
func (t *Tracer) Record(ctx context.Context, zs ZipkinSpan) error {
	if t.Recorder == nil {
		return nil
	}
	data, err := json.Marshal(zs)
	if err != nil {
		return err
	}

	timeout := t.MaxRecordTimeout
	if timeout <= 0 {
		timeout = DefaultMaxRecordTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	err = t.Recorder.Send(ctx, data)
	if t.Logger != nil {
		if errors.As(err, new(mqsend.MessageTooLargeError)) {
			t.Logger(fmt.Sprintf(
				"Span is too big, max allowed size is %d. This can be caused by an excess amount of tags. Error: %v",
				MaxSpanSize,
				err,
			))
		}
		if errors.As(err, new(mqsend.TimedOutError)) {
			t.Logger(
				"Trace queue is full. Is trace sidecar healthy? Error: " + err.Error(),
			)
		}
	}
	return err
}

// StartSpan implements opentracing.Tracer.
//
// For opentracing.StartSpanOptions,
// it only support the following options and will ignore all others:
//
// - ChildOfRef (in which case the parent span must be of type *Span)
//
// - StartTime
//
// - Tags
//
// It supports additional StartSpanOptions defined in this package.
//
// Please note that trying to set span type via opentracing-go/ext package won't
// work, please use SpanTypeOption defined in this package instead.
func (t *Tracer) StartSpan(operationName string, opts ...opentracing.StartSpanOption) opentracing.Span {
	var sso StartSpanOptions
	for _, opt := range opts {
		sso.Apply(opt)
	}

	span := newSpan(t, operationName, sso.Type)
	span.component = sso.ComponentName
	if !sso.OpenTracingOptions.StartTime.IsZero() {
		span.trace.start = sso.OpenTracingOptions.StartTime
	}
	if parent := findFirstParentReference(sso.OpenTracingOptions.References); parent != nil {
		parent.initChildSpan(span)
	}
	for key, value := range sso.OpenTracingOptions.Tags {
		span.SetTag(key, value)
	}

	return span
}

// Inject implements opentracing.Tracer.
//
// Currently it always return opentracing.ErrInvalidCarrier as the error.
func (t *Tracer) Inject(sm opentracing.SpanContext, format interface{}, carrier interface{}) error {
	return opentracing.ErrInvalidCarrier
}

// Extract implements opentracing.Tracer.
//
// Currently it always return opentracing.ErrInvalidCarrier as the error.
func (t *Tracer) Extract(format interface{}, carrier interface{}) (opentracing.SpanContext, error) {
	return nil, opentracing.ErrInvalidCarrier
}

func findFirstParentReference(refs []opentracing.SpanReference) *Span {
	for _, s := range refs {
		if s.Type == opentracing.ChildOfRef {
			// Note that we only support using our type as the parent right now.
			if span, ok := s.ReferencedContext.(*Span); ok {
				return span
			}
		}
	}
	return nil
}
