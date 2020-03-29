package tracing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	opentracing "github.com/opentracing/opentracing-go"

	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/mqsend"
	"github.com/reddit/baseplate.go/randbp"
	"github.com/reddit/baseplate.go/runtimebp"
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

// Register an empty Tracer implementation as opentracing's global tracer.
func init() {
	opentracing.SetGlobalTracer(&globalTracer)
}

var globalTracer = Tracer{logger: log.NopWrapper}

// A Tracer creates and manages spans.
type Tracer struct {
	sampleRate       float64
	recorder         mqsend.MessageQueue
	logger           log.Wrapper
	endpoint         ZipkinEndpointInfo
	maxRecordTimeout time.Duration
}

// TracerConfig are the configuration values to be used in InitGlobalTracer.
type TracerConfig struct {
	// The name of the service that will be attached to every span.
	ServiceName string

	// SampleRate should be in the range of [0, 1].
	// When SampleRate >= 1, all traces will be recoreded;
	// When SampleRate <= 0, none of the traces will be recorded,
	// except the ones with debug flag set.
	//
	// Please note that SampleRate only affect top level spans created inside this
	// service. For most services the sample status will be inherited from the
	// headers from the client.
	SampleRate float64

	// Logger, if non-nil, will be used to log additional informations Record
	// returned certain errors.
	Logger log.Wrapper

	// The max timeout applied to Record function.
	//
	// If the passed in context object has an earlier deadline set,
	// that deadline will be respected instead.
	//
	// If MaxRecordTimeout <= 0, DefaultMaxRecordTimeout will be used.
	MaxRecordTimeout time.Duration

	// The name of the message queue to be used to actually send sampled spans to
	// backend service (requires baseplate.py tracing publishing sidecar with the
	// same name configured).
	//
	// QueueName should not contain "traces-" prefix, it will be auto added.
	//
	// If QueueName is empty, no spans will be sampled,
	// including the ones with debug flag set.
	QueueName string

	// In test code,
	// this field can be used to set the message queue the tracer publishes to,
	// usually an *mqsend.MockMessageQueue.
	//
	// This field will be ignored when QueueName is non-empty,
	// to help avoiding footgun prod code.
	//
	// DO NOT USE IN PROD CODE.
	TestOnlyMockMessageQueue mqsend.MessageQueue
}

// InitGlobalTracer initializes opentracing's global tracer.
//
// This function will try to get the first local IPv4 address of this machine
// as part of the span information send to the backend service.
// If it fails to do so, UndefinedIP will be used instead,
// and the error will be logged if logger is non-nil.
func InitGlobalTracer(cfg TracerConfig) error {
	if cfg.QueueName != "" {
		recorder, err := mqsend.OpenMessageQueue(mqsend.MessageQueueConfig{
			Name:           QueueNamePrefix + cfg.QueueName,
			MaxQueueSize:   MaxQueueSize,
			MaxMessageSize: MaxSpanSize,
		})
		if err != nil {
			return err
		}
		globalTracer.recorder = recorder
	} else {
		globalTracer.recorder = cfg.TestOnlyMockMessageQueue
	}

	globalTracer.sampleRate = cfg.SampleRate

	logger := cfg.Logger
	if logger == nil {
		logger = log.NopWrapper
	}
	globalTracer.logger = logger

	timeout := cfg.MaxRecordTimeout
	if timeout <= 0 {
		timeout = DefaultMaxRecordTimeout
	}
	globalTracer.maxRecordTimeout = timeout

	ip, err := runtimebp.GetFirstIPv4()
	if err != nil {
		logger(`Unable to get local ip address: ` + err.Error())
	}
	globalTracer.endpoint = ZipkinEndpointInfo{
		ServiceName: cfg.ServiceName,
		IPv4:        ip,
	}

	opentracing.SetGlobalTracer(&globalTracer)
	return nil
}

// Close closes the tracer's reporting.
//
// After Close is called, no more spans will be sampled.
func (t *Tracer) Close() error {
	if t.recorder == nil {
		return nil
	}
	err := t.recorder.Close()
	t.recorder = nil
	return err
}

// Record records a span with the Recorder.
//
// Span.Stop(), Span.Finish(), and Span.FinishWithOptions() call this function
// automatically.
// In most cases that should be enough and you should not call this function
// directly.
func (t *Tracer) Record(ctx context.Context, zs ZipkinSpan) error {
	if t.recorder == nil {
		return nil
	}
	data, err := json.Marshal(zs)
	if err != nil {
		return err
	}

	timeout := t.maxRecordTimeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	err = t.recorder.Send(ctx, data)
	if t.logger != nil {
		if errors.As(err, new(mqsend.MessageTooLargeError)) {
			t.logger(fmt.Sprintf(
				"Span is too big, max allowed size is %d. This can be caused by an excess amount of tags. Error: %v",
				MaxSpanSize,
				err,
			))
		}
		if errors.As(err, new(mqsend.TimedOutError)) {
			t.logger(
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
// If the new span's type is server,
// all registered CreateServerSpanHooks will be called as well.
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
	parent := findFirstParentReference(sso.OpenTracingOptions.References)
	if parent != nil {
		parent.initChildSpan(span)
	} else {
		span.trace.traceID = randbp.R.Uint64()
		span.trace.sampled = randbp.ShouldSampleWithRate(t.sampleRate)
	}

	switch span.spanType {
	case SpanTypeServer:
		// Special handlings for server spans. See also: Span.initChildSpan.
		onCreateServerSpan(span)
		span.onStart()
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

func (t *Tracer) getLogger() log.Wrapper {
	if t.logger != nil {
		return t.logger
	}
	return log.NopWrapper
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

// CloseTracer tries to cast opentracing.GlobalTracer() into *Tracer, and calls
// its Close function.
//
// See Tracer.Close for more details.
func CloseTracer() error {
	if tracer, ok := opentracing.GlobalTracer().(*Tracer); ok {
		return tracer.Close()
	}
	return nil
}
