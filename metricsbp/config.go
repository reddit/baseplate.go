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

	// Tags are the base tags that will be applied to all metrics.
	Tags Tags `yaml:"tags"`

	// DEPRECATED: There's not really a reason to sample counters in Baseplate.go
	// as they are always aggregated in memory. This config will be removed in a
	// future release.
	//
	// CounterSampleRate is the fraction of counters that you want to send to your
	// metrics backend.
	//
	// Optional, defaults to 1 (100%).
	CounterSampleRate *float64 `yaml:"counterSampleRate"`

	// HistogramSampleRate is the fraction of histograms (including timings) that
	// you want to send to your metrics  backend.
	//
	// Optional, defaults to 1 (100%).
	HistogramSampleRate *float64 `yaml:"histogramSampleRate"`

	// RunSysStats indicates that you want to publish system stats.
	//
	// Optional, defaults to false.
	RunSysStats bool `yaml:"runSysStats"`
}

// InitFromConfig initializes the global metricsbp.M with the given context and
// Config and returns an io.Closer to use to close out the metrics client when
// your server exits.
//
// It also registers CreateServerSpanHook and ConcurrencyCreateServerSpanHook
// with the global tracing hook registry.
func InitFromConfig(ctx context.Context, cfg Config) io.Closer {
	M = NewStatsd(ctx, StatsdConfig{
		CounterSampleRate:   cfg.CounterSampleRate,
		HistogramSampleRate: cfg.HistogramSampleRate,
		Prefix:              cfg.Namespace,
		Address:             cfg.Endpoint,
		LogLevel:            log.ErrorLevel,
		Tags:                cfg.Tags,
	})
	tracing.RegisterCreateServerSpanHooks(CreateServerSpanHook{Metrics: M})
	if cfg.RunSysStats {
		M.RunSysStats()
	}
	return M
}
