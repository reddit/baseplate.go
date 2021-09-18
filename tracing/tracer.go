package tracing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/opentracing/opentracing-go"

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
)

func init() {
	// Register an empty Tracer implementation as opentracing's global tracer.
	opentracing.SetGlobalTracer(&globalTracer)
	// Init the global allow-list
	SetMetricsTagsAllowList(nil)
}

var globalTracer = Tracer{logger: log.NopWrapper}

// A Tracer creates and manages spans.
type Tracer struct {
	sampleRate       float64
	recorder         mqsend.MessageQueue
	logger           log.Wrapper
	endpoint         ZipkinEndpointInfo
	maxRecordTimeout time.Duration
	useHex           bool
}

// InitGlobalTracer initializes opentracing's global tracer.
//
// This function will try to get the first local IPv4 address of this machine
// as part of the span information send to the backend service.
// If it fails to do so, UndefinedIP will be used instead,
// and the error will be logged if logger is non-nil.
func InitGlobalTracer(cfg Config) error {
	var tracer Tracer
	if cfg.QueueName != "" {
		if cfg.MaxQueueSize <= 0 || cfg.MaxQueueSize > MaxQueueSize {
			cfg.MaxQueueSize = MaxQueueSize
		}
		recorder, err := mqsend.OpenMessageQueue(mqsend.MessageQueueConfig{
			Name:           QueueNamePrefix + cfg.QueueName,
			MaxQueueSize:   cfg.MaxQueueSize,
			MaxMessageSize: MaxSpanSize,
		})
		if err != nil {
			return err
		}
		tracer.recorder = recorder
	} else {
		tracer.recorder = cfg.TestOnlyMockMessageQueue
	}

	tracer.sampleRate = cfg.SampleRate
	tracer.useHex = cfg.UseHex

	logger := cfg.Logger
	if logger == nil {
		logger = log.NopWrapper
	}
	tracer.logger = logger

	tracer.maxRecordTimeout = cfg.MaxRecordTimeout

	ip, err := runtimebp.GetFirstIPv4()
	if err != nil {
		logger(context.Background(), `Unable to get local ip address: `+err.Error())
	}
	tracer.endpoint = ZipkinEndpointInfo{
		ServiceName: cfg.Namespace,
		IPv4:        ip,
	}

	globalTracer = tracer
	opentracing.SetGlobalTracer(&globalTracer)
	return nil
}

type closer struct{}

func (closer) Close() error {
	return CloseTracer()
}

// InitGlobalTracerWithCloser is the combination of InitGlobalTracer and
// CloseTracer.
//
// After successful initialization,
// the returned Closer would delegate to CloseTracer upon called.
func InitGlobalTracerWithCloser(cfg Config) (io.Closer, error) {
	if err := InitGlobalTracer(cfg); err != nil {
		return nil, err
	}
	return closer{}, nil
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

	if ctx.Err() != nil {
		// The request context is already canceled.
		// Use background to make sure we are still able to send out the span,
		ctx = context.Background()
	}

	timeout := t.maxRecordTimeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	err = t.recorder.Send(ctx, data)
	if errors.As(err, new(mqsend.MessageTooLargeError)) {
		t.logger.Log(ctx, fmt.Sprintf(
			"Span is too big, max allowed size is %d. This can be caused by an excess amount of tags. Error: %v",
			MaxSpanSize,
			err,
		))
	}
	if errors.As(err, new(mqsend.TimedOutError)) {
		t.logger.Log(
			ctx,
			"Trace queue is full. Is trace sidecar healthy? Error: "+err.Error(),
		)
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
	if !sso.OpenTracingOptions.StartTime.IsZero() {
		span.trace.start = sso.OpenTracingOptions.StartTime
	}

	parent := findFirstParentReference(sso.OpenTracingOptions.References)
	if parent != nil {
		parent.initChildSpan(span)
	} else {
		span.trace.traceID = t.newTraceID()
		span.trace.sampled = randbp.ShouldSampleWithRate(t.sampleRate)
		initRootSpan(context.Background(), span)
	}

	if span.spanType == SpanTypeServer {
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

func (t *Tracer) newTraceID() string {
	if t.useHex {
		// For traces we just combine two 64-bit hex ids to get a 128-bit hex id.
		return hexID64() + hexID64()
	}
	return decID64()
}

func (t *Tracer) newSpanID() string {
	if t.useHex {
		return hexID64()
	}
	return decID64()
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

// hexID64 generates 64-bit hex id.
func hexID64() string {
	return fmt.Sprintf("%016x", randbp.R.Uint64())
}

// decID64 generates 64-bit dec id, excluding 0.
func decID64() string {
	return strconv.FormatUint(nonZeroRandUint64(), 10)
}
