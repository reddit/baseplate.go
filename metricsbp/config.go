package metricsbp

import (
	"context"
	"io"

	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/tracing"
)

// Config is the confuration struct for the metricsbp package.
//
// Can be deserialized from YAML.
type Config struct {
	// Namespace is the standard prefix applied to all of your metrics, it should
	// include the name of your service.
	Namespace string `yaml:"namespace"`

	// Endpoint is the endpoint for your metrics backend.
	Endpoint string `yaml:"endpoint"`

	// CounterSampleRate is the fraction of counters that you want to send to your
	// metrics backend.
	//
	// Optional, defaults to 1.0
	CounterSampleRate *float64 `yaml:"counterSampleRate"`

	// CounterSampleRate is the fraction of histograms that you want to send to
	// your metrics  backend.
	//
	// Optional, defaults to 1.0
	HistogramSampleRate *float64 `yaml:"histogramSampleRate"`
}

// InitFromConfig initializes the global metricsbp.M with the given context and
// Config and returns an io.Closer to use to close out the metrics client when
// your server exits.
//
// It also registers CreateServerSpanHook with the global tracing hook registry.
func InitFromConfig(ctx context.Context, cfg Config) io.Closer {
	M = NewStatsd(ctx, StatsdConfig{
		CounterSampleRate:   cfg.CounterSampleRate,
		HistogramSampleRate: cfg.HistogramSampleRate,
		Prefix:              cfg.Namespace,
		Address:             cfg.Endpoint,
		LogLevel:            log.ErrorLevel,
	})
	tracing.RegisterCreateServerSpanHooks(CreateServerSpanHook{Metrics: M})
	return M
}
