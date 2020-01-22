package metricsbp

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/reddit/baseplate.go/log"

	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/influxstatsd"
)

// ReporterTickerInterval is the interval the reporter sends data to statsd
// server. Default is one minute.
var ReporterTickerInterval = time.Minute

// Statsd defines a statsd reporter (with influx extension) and the root of the
// metrics.
//
// It can be used to create metrics,
// and also maintains the background reporting goroutine,
//
// Please use NewStatsd to initialize it.
type Statsd struct {
	Statsd *influxstatsd.Influxstatsd

	sampleRate float64

	reporter *time.Ticker
	wg       *sync.WaitGroup

	ctx    context.Context
	cancel context.CancelFunc
}

// StatsdConfig is the configs used in NewStatsd.
type StatsdConfig struct {
	// Prefix is the common metrics path prefix shared by all metrics managed by
	// (created from) this Metrics object.
	//
	// If it's not ending with a period ("."), a period will be added.
	Prefix string

	// DefaultSampleRate is the default reporting sample rate used when creating
	// metrics.
	DefaultSampleRate float64

	// Address is the UDP address (in "host:port" format) of the statsd service.
	//
	// It could be empty string, in which case we won't start the background
	// reporting goroutine.
	//
	// When Address is the empty string,
	// the Statsd object and the metrics created under it will not be reported
	// anywhere,
	// so it can be used in lieu of discarded metrics in test code.
	// But the metrics are still stored in memory,
	// so it shouldn't be used in lieu of discarded metrics in prod code.
	Address string

	// The log level used by the reporting goroutine.
	LogLevel log.Level

	// Labels are the labels/tags to be attached to every metrics created
	// from this Statsd object. For labels/tags only needed by some metrics,
	// use Counter/Gauge/Timing.With() instead.
	Labels map[string]string
}

// NewStatsd creates a Statsd object.
//
// It also starts the background reporting goroutine.
func NewStatsd(ctx context.Context, cfg StatsdConfig) Statsd {
	prefix := cfg.Prefix
	if prefix != "" && !strings.HasSuffix(prefix, ".") {
		prefix = prefix + "."
	}
	labels := make([]string, 0, len(cfg.Labels)*2)
	for k, v := range cfg.Labels {
		labels = append(labels, k, v)
	}
	st := Statsd{
		Statsd:     influxstatsd.New(prefix, log.KitLogger(cfg.LogLevel), labels...),
		sampleRate: cfg.DefaultSampleRate,
	}

	if cfg.Address != "" {
		st.reporter = time.NewTicker(ReporterTickerInterval)
		st.ctx, st.cancel = context.WithCancel(ctx)
		st.wg = new(sync.WaitGroup)
		st.wg.Add(1)
		go func() {
			defer st.wg.Done()
			st.Statsd.SendLoop(st.ctx, st.reporter.C, "udp", cfg.Address)
		}()
	}

	return st
}

// StopReporting stops the background reporting goroutine.
//
// Note that cancelling the context passed into NewStatsd would also stop the
// background reporting goroutine,
// but that won't stop the ticker and will cause resource leak.
//
// It's OK to call StopReporting multiple times,
// or after cancelling the context passed into NewStatsd.
func (st Statsd) StopReporting() {
	if st.reporter == nil {
		return
	}

	st.cancel()
	st.reporter.Stop()
	st.wg.Wait()
}

// Counter returns a counter metrics to the name.
//
// It uses the DefaultSampleRate used to create Statsd object.
// If you need a different sample rate,
// you could use st.Statsd.NewCounter instead.
func (st Statsd) Counter(name string) metrics.Counter {
	return st.Statsd.NewCounter(name, st.sampleRate)
}

// Histogram returns a histogram metrics to the name.
//
// It uses the DefaultSampleRate used to create Statsd object.
// If you need a different sample rate,
// you could use st.Statsd.NewTiming instead.
func (st Statsd) Histogram(name string) metrics.Histogram {
	return st.Statsd.NewTiming(name, st.sampleRate)
}

// Gauge returns a gauge metrics to the name.
//
// It's a shortcut to st.Statsd.NewGauge(name).
func (st Statsd) Gauge(name string) metrics.Gauge {
	return st.Statsd.NewGauge(name)
}
