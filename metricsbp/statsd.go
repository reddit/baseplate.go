package metricsbp

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/reddit/baseplate.go/log"

	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/influxstatsd"
)

const epsilon = 1e-9

// ReporterTickerInterval is the interval the reporter sends data to statsd
// server. Default is one minute.
var ReporterTickerInterval = time.Minute

// M is short for "Metrics".
//
// This is the global Statsd to use.
// It's pre-initialized with one that does not send metrics anywhere,
// so it won't cause panic even if you don't initialize it before using it
// (for example, it's safe to be used in test code).
//
// But in production code you should still properly initialize it to actually
// send your metrics to your statsd collector,
// usually early in your main function:
//
//     func main() {
//       flag.Parse()
//       ctx, cancel := context.WithCancel(context.Background())
//       defer cancel()
//       metricsbp.M = metricsbp.NewStatsd{
//         ctx,
//         metricsbp.StatsdConfig{
//           ...
//         },
//       }
//       metricsbp.M.RunSysStats()
//       ...
//     }
//
//     func someOtherFunction() {
//       ...
//       metricsbp.M.Counter("my-counter").Add(1)
//       ...
//     }
var M = NewStatsd(context.Background(), StatsdConfig{})

// Statsd defines a statsd reporter (with influx extension) and the root of the
// metrics.
//
// It can be used to create metrics,
// and also maintains the background reporting goroutine,
//
// Please use NewStatsd to initialize it.
//
// When a *Statsd is nil,
// any function calls to it will fallback to use M instead,
// so they are safe to use (unless M was explicitly overridden as nil),
// but accessing the fields will still cause panic.
// For example:
//
//     st := (*metricsbp.Statsd)(nil)
//     st.Counter("my-counter").Add(1) // does not panic unless metricsbp.M is nil
//     st.Statsd.NewCounter("my-counter", 0.5).Add(1) // panics
type Statsd struct {
	Statsd *influxstatsd.Influxstatsd

	ctx                 context.Context
	counterSampleRate   float64
	histogramSampleRate float64
}

// StatsdConfig is the configs used in NewStatsd.
type StatsdConfig struct {
	// Prefix is the common metrics path prefix shared by all metrics managed by
	// (created from) this Metrics object.
	//
	// If it's not ending with a period ("."), a period will be added.
	Prefix string

	// The reporting sample rate used when creating counters and
	// timings/histograms, respectively.
	//
	// For user convenience,
	// we actually treat zero values (within 1e-9 since it's float) as 1 (100%),
	// and <-1e-9 as 0 (0%).
	CounterSampleRate   float64
	HistogramSampleRate float64

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

// counterSampleRate treats 0 (abs(rate) < epsilon) as 1, and <-epsilon as 0.
func convertSampleRate(rate float64) float64 {
	if math.Abs(rate) < epsilon {
		return 1
	}
	if rate < 0 {
		return 0
	}
	return rate
}

// NewStatsd creates a Statsd object.
//
// It also starts a background reporting goroutine when Address is not empty.
// The goroutine will be stopped when the passed in context is canceled.
//
// NewStatsd never returns nil.
func NewStatsd(ctx context.Context, cfg StatsdConfig) *Statsd {
	prefix := cfg.Prefix
	if prefix != "" && !strings.HasSuffix(prefix, ".") {
		prefix = prefix + "."
	}
	labels := make([]string, 0, len(cfg.Labels)*2)
	for k, v := range cfg.Labels {
		labels = append(labels, k, v)
	}
	st := &Statsd{
		Statsd:              influxstatsd.New(prefix, log.KitLogger(cfg.LogLevel), labels...),
		ctx:                 ctx,
		counterSampleRate:   convertSampleRate(cfg.CounterSampleRate),
		histogramSampleRate: convertSampleRate(cfg.HistogramSampleRate),
	}

	if cfg.Address != "" {
		go func() {
			ticker := time.NewTicker(ReporterTickerInterval)
			defer ticker.Stop()

			st.Statsd.SendLoop(ctx, ticker.C, "udp", cfg.Address)
		}()
	}

	return st
}

// Counter returns a counter metrics to the name,
// with sample rate inherited from StatsdConfig.
func (st *Statsd) Counter(name string) metrics.Counter {
	st = st.fallback()
	counter := st.Statsd.NewCounter(name, st.counterSampleRate)
	if st.counterSampleRate >= 1 {
		return counter
	}
	return SampledCounter{
		Counter: counter,
		Rate:    st.counterSampleRate,
	}
}

// Histogram returns a histogram metrics to the name with no specific unit,
// with sample rate inherited from StatsdConfig.
func (st *Statsd) Histogram(name string) metrics.Histogram {
	st = st.fallback()
	histogram := st.Statsd.NewHistogram(name, st.histogramSampleRate)
	if st.histogramSampleRate >= 1 {
		return histogram
	}
	return SampledHistogram{
		Histogram: histogram,
		Rate:      st.histogramSampleRate,
	}
}

// Timing returns a histogram metrics to the name with milliseconds as the unit,
// with sample rate inherited from StatsdConfig.
func (st *Statsd) Timing(name string) metrics.Histogram {
	st = st.fallback()
	histogram := st.Statsd.NewTiming(name, st.histogramSampleRate)
	if st.histogramSampleRate >= 1 {
		return histogram
	}
	return SampledHistogram{
		Histogram: histogram,
		Rate:      st.histogramSampleRate,
	}
}

// Gauge returns a gauge metrics to the name.
//
// It's a shortcut to st.Statsd.NewGauge(name).
func (st *Statsd) Gauge(name string) metrics.Gauge {
	st = st.fallback()
	return st.Statsd.NewGauge(name)
}

func (st *Statsd) fallback() *Statsd {
	if st == nil {
		return M
	}
	return st
}

// Ctx provides a read-only access to the context object this Statsd holds.
//
// It's useful when you need to implement your own goroutine to report some
// metrics (usually gauges) periodically,
// and be able to stop that goroutine gracefully.
// For example:
//
//     func reportGauges() {
//       gauge := metricsbp.M.Gauge("my-gauge")
//       go func() {
//         ticker := time.NewTicker(time.Minute)
//         defer ticker.Stop()
//
//         for {
//           select {
//           case <- metricsbp.M.Ctx().Done():
//             return
//           case <- ticker.C:
//             gauge.Set(getValue())
//           }
//         }
//       }
//     }
func (st *Statsd) Ctx() context.Context {
	return st.ctx
}
