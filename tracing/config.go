package tracing

import (
	"io"
	"time"

	"github.com/reddit/baseplate.go/log"
)

// Config is the confuration struct for the tracing package.
//
// Can be deserialized from YAML.
type Config struct {
	// Namespace the name of your service.
	Namespace string `yaml:"namespace"`

	// QueueName is the name of the POSIX queue your service publishes traces to
	// to be read by a trace publisher sidecar.
	QueueName string `yaml:"queueName"`

	// The max size of the message queue (number of messages).
	MaxQueueSize int64 `yaml:"maxQueueSize"`

	// RecordTimeout is the timeout on writing a trace to the POSIX queue.
	RecordTimeout time.Duration `yaml:"recordTimeout"`

	// SampleRate is the % of new trace's to sample.
	SampleRate float64 `yaml:"sampleRate"`

	// Generate UUID instead of uint64 for new trace/span IDs.
	// NOTE: Only enable this if you know all your upstream services can handle
	// UUID trace/span IDs (Baseplate.go v0.8.0+ or Baseplate.py v2.0.0+).
	UseUUID bool `yaml:"useUUID"`
}

// InitFromConfig initializes the global tracer using the given Config and
// also registers the ErrorReporterCreateServerSpanHook with the global span
// hook registry.
//
// It returns an io.Closer that can be used to close out the tracer when the
// server is done executing.
func InitFromConfig(cfg Config) (io.Closer, error) {
	closer, err := InitGlobalTracerWithCloser(TracerConfig{
		ServiceName:      cfg.Namespace,
		SampleRate:       cfg.SampleRate,
		MaxRecordTimeout: cfg.RecordTimeout,
		QueueName:        cfg.QueueName,
		MaxQueueSize:     cfg.MaxQueueSize,
		UseUUID:          cfg.UseUUID,
		Logger:           log.ErrorWithSentryWrapper(),
	})
	if err != nil {
		return nil, err
	}

	RegisterCreateServerSpanHooks(ErrorReporterCreateServerSpanHook{})
	return closer, nil
}
