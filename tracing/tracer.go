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
func (t Tracer) NewTrace(name string) *Span {
	span := newSpan(&t, name, SpanTypeLocal)
	span.traceID = rand.Uint64()
	span.sampled = randbp.ShouldSampleWithRate(t.SampleRate)
	span.startSpan()
	return span
}

// Record records a span with the Recorder.
//
// Span.End() calls this function automatically.
// In most cases that should be enough and you should not call this function
// directly.
func (t Tracer) Record(ctx context.Context, span *Span) error {
	if !span.ShouldSample() {
		return nil
	}
	if t.Recorder == nil {
		return nil
	}
	data, err := json.Marshal(span.ToZipkinSpan())
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
