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

	// RecordTimeout is the timeout on writing a trace to the POSIX queue.
	RecordTimeout time.Duration `yaml:"recordTimeout"`

	// SampleRate is the % of new trace's to sample.
	SampleRate float64 `yaml:"sampleRate"`
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
		Logger:           log.ErrorWithSentryWrapper(),
	})
	if err != nil {
		return nil, err
	}

	RegisterCreateServerSpanHooks(ErrorReporterCreateServerSpanHook{})
	return closer, nil
}
