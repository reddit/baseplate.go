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

	// HistogramSampleRate is the fraction of histograms (including timings) that
	// you want to send to your metrics  backend.
	//
	// Optional, defaults to 1 (100%).
	HistogramSampleRate *float64 `yaml:"histogramSampleRate"`

	// When Endpoint is configured,
	// BufferSize can be used to buffer writes to statsd collector together.
	//
	// Set it to an appropriate number will reduce the number of UDP messages sent
	// to the statsd collector. (Recommendation: 4KB)
	//
	// It's guaranteed that every single UDP message will not exceed BufferSize,
	// unless a single metric line exceeds it
	// (usually around 100 bytes depending on the length of the metric path).
	//
	// When it's 0 (default), DefaultBufferSize will be used.
	//
	// To disable buffering, set it to negative value (e.g. -1) or a value smaller
	// than a single statsd metric line (usually around 100, so 1 works),
	// on ReporterTickerInterval we will write one UDP message per metric to the
	// statsd collector.
	BufferSize int `yaml:"bufferSize"`

	// The log level used by the reporting goroutine.
	LogLevel log.Level `yaml:"logLevel"`

	// RunSysStats indicates that you want to publish system stats.
	//
	// Optional, defaults to false.
	RunSysStats bool `yaml:"runSysStats"`

	// ReportServerConnectionCount indicates that you want to publish
	// a counter for the number of clients connected to the server
	//
	// Optional, defaults to false.
	ReportServerConnectionCount bool `yaml:"ReportServerConnectionCount"`
}

// InitFromConfig initializes the global metricsbp.M with the given context and
// Config and returns an io.Closer to use to close out the metrics client when
// your server exits.
//
// It also registers CreateServerSpanHook and ConcurrencyCreateServerSpanHook
// with the global tracing hook registry.
func InitFromConfig(ctx context.Context, cfg Config) io.Closer {
	M = NewStatsd(ctx, cfg)
	tracing.RegisterCreateServerSpanHooks(CreateServerSpanHook{Metrics: M})
	if cfg.RunSysStats {
		M.RunSysStats()
	}
	return M
}
