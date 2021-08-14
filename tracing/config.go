package tracing

import (
	"io"
	"time"

	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/mqsend"
)

// Config is the configuration struct for the tracing package containing
// configuration values to be used in InitGlobalTracer.
//
// Can be deserialized from YAML.
type Config struct {
	// The name of the service that will be attached to every span.
	Namespace string `yaml:"namespace"`

	// SampleRate should be in the range of [0, 1].
	// When SampleRate >= 1, all traces will be recoreded;
	// When SampleRate <= 0, none of the traces will be recorded,
	// except the ones with debug flag set.
	//
	// Please note that SampleRate only affect top level spans created inside this
	// service. For most services the sample status will be inherited from the
	// headers from the client.
	SampleRate float64 `yaml:"sampleRate"`

	// Logger, if non-nil, will be used to log additional informations Record
	// returned certain errors.
	Logger log.Wrapper `yaml:"logger"`

	// The max timeout applied to Record function.
	//
	// If the passed in context object has an earlier deadline set,
	// that deadline will be respected instead.
	//
	// If MaxRecordTimeout <= 0,
	// Record function would run in non-blocking mode,
	// that it fails immediately if the queue is full.
	MaxRecordTimeout time.Duration `yaml:"recordTimeout"`

	// The name of the message queue to be used to actually send sampled spans to
	// backend service (requires Baseplate.py tracing publishing sidecar with the
	// same name configured).
	//
	// QueueName should not contain "traces-" prefix, it will be auto added.
	//
	// If QueueName is empty, no spans will be sampled,
	// including the ones with debug flag set.
	QueueName string `yaml:"queueName"`

	// The max size of the message queue (number of messages).
	//
	// If it <=0 or > MaxQueueSize (the constant, 10000),
	// MaxQueueSize constant will be used instead.
	//
	// This is only used when QueueName is non-empty.
	MaxQueueSize int64 `yaml:"maxQueueSize"`

	// If UseHex is set to true, when generating new trace/span IDs we will use
	// hex instead of dec uint64.
	//
	// You should only set this to true if you know all of your upstream servers
	// can handle hex trace ids (Baseplate.go v0.8.0+ or Baseplate.py v2.0.0+).
	UseHex bool `yaml:"useHex"`

	// In test code,
	// this field can be used to set the message queue the tracer publishes to,
	// usually an *mqsend.MockMessageQueue.
	//
	// This field will be ignored when QueueName is non-empty,
	// to help avoiding footgun prod code.
	//
	// DO NOT USE IN PROD CODE.
	TestOnlyMockMessageQueue mqsend.MessageQueue `yaml:"-"`
}

// InitFromConfig initializes the global tracer using the given Config and
// also registers the ErrorReporterCreateServerSpanHook with the global span
// hook registry.
//
// It returns an io.Closer that can be used to close out the tracer when the
// server is done executing.
func InitFromConfig(cfg Config) (io.Closer, error) {
	closer, err := InitGlobalTracerWithCloser(cfg)
	if err != nil {
		return nil, err
	}

	RegisterCreateServerSpanHooks(ErrorReporterCreateServerSpanHook{})
	return closer, nil
}
