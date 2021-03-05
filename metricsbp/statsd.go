package metricsbp

import (
	"context"
	"io"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/influxstatsd"
	"github.com/go-kit/kit/util/conn"

	"github.com/reddit/baseplate.go/log"
)

// Default values to be used in the config.
const (
	// DefaultSampleRate is the default value to be used when *SampleRate in
	// StatsdConfig is nil (zero value).
	DefaultSampleRate = 1

	// DefaultBufferSize is the default value to be used when BufferSize in
	// StatsdConfig is 0.
	DefaultBufferSize = 4096
)

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
// It supports metrics tags in Influxstatsd format.
//
// Please use NewStatsd to initialize it.
//
// When a *Statsd is nil,
// any function calls to it will fallback to use M instead,
// so they are gonna be safe to use (unless M was explicitly overridden as nil).
// For example:
//
//     st := (*metricsbp.Statsd)(nil)
//     st.Counter("my-counter").Add(1) // does not panic unless metricsbp.M is nil
type Statsd struct {
	statsd *influxstatsd.Influxstatsd

	cfg                 StatsdConfig
	ctx                 context.Context
	cancel              context.CancelFunc
	counterSampleRate   float64
	histogramSampleRate float64
	writer              *bufferedWriter

	activeRequests int64
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
	// DefaultSampleRate will be used when they are nil (zero value).
	//
	// Use Float64Ptr to convert literals or other values that you can't get the
	// pointer directly.
	//
	// To override global sample rate set here for particular counters/histograms,
	// use CounterWithRate/HistogramWithRate/TimingWithRate.
	CounterSampleRate   *float64
	HistogramSampleRate *float64

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

	// When Address is configured,
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
	BufferSize int

	// The log level used by the reporting goroutine.
	LogLevel log.Level

	// Tags are the tags to be attached to every metrics created from this Statsd
	// object. For tags only needed by some metrics, use Counter/Gauge/Timing.With()
	// instead.
	Tags Tags
}

func convertSampleRate(rate *float64) float64 {
	if rate == nil {
		return DefaultSampleRate
	}
	return *rate
}

// Float64Ptr converts float64 value into pointer.
func Float64Ptr(v float64) *float64 {
	return &v
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
	tags := cfg.Tags.AsStatsdTags()
	kitlogger := log.KitLogger(cfg.LogLevel)
	st := &Statsd{
		statsd:              influxstatsd.New(prefix, kitlogger, tags...),
		cfg:                 cfg,
		counterSampleRate:   convertSampleRate(cfg.CounterSampleRate),
		histogramSampleRate: convertSampleRate(cfg.HistogramSampleRate),
	}
	st.ctx, st.cancel = context.WithCancel(ctx)

	if cfg.Address != "" {
		if cfg.BufferSize == 0 {
			cfg.BufferSize = DefaultBufferSize
		}
		st.writer = newBufferedWriter(
			conn.NewDefaultManager("udp", cfg.Address, kitlogger),
			cfg.BufferSize,
		)
		go func() {
			ticker := time.NewTicker(ReporterTickerInterval)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					st.writer.doWrite(st.statsd, kitlogger)
				case <-st.ctx.Done():
					// Flush one more time before returning.
					st.writer.doWrite(st.statsd, kitlogger)
					return
				}
			}
		}()
	}

	return st
}

// RateArgs defines the args used by -WithRate functions.
type RateArgs struct {
	// Name of the metric, required.
	Name string

	// Sampling rate, required.
	//
	// If AlreadySampledAt is nil,
	// this controls both the sample rate reported to statsd and the rate we use
	// for reporting the metrics.
	//
	// If AlreadySampledAt is non-nil,
	// The sample rate reported to statsd will be Rate*AlreadySampledAt,
	// and Rate controls how we randomly report this metric.
	// It's useful when you are reporting a metric from an already sampled code
	// block, for example:
	//
	//     const rate = 0.01
	//     if randbp.ShouldSampleWithRate(rate) {
	//       if err := myFancyWork(); err != nil {
	//         metricsbp.M.CounterWithRate(metricsbp.RateArgs{
	//           Name: "my.fancy.work.errors",
	//           // 100% report it because we are already sampling it.
	//           Rate: 1,
	//           // but adjust the reporting rate to the actual sample rate.
	//           AlreadySampledAt: metricsbp.Float64Ptr(rate),
	//         }).Add(1)
	//       }
	//     }
	Rate float64

	// Optional. Default to 1 (100%) if it's nil.
	// See the comment on Rate for more details.
	//
	// It will be treated as 1 if >=1, and be trated as 0 if <=0.
	AlreadySampledAt *float64
}

// ReportingRate returns the reporting rate according to the args.
func (ra RateArgs) ReportingRate() float64 {
	if ra.AlreadySampledAt == nil {
		return ra.Rate
	}
	rate := *ra.AlreadySampledAt
	if rate >= 1 {
		return ra.Rate
	}
	if rate <= 0 {
		return 0
	}
	return rate * ra.Rate
}

// Counter returns a counter metrics to the name,
// with sample rate inherited from StatsdConfig.
func (st *Statsd) Counter(name string) metrics.Counter {
	st = st.fallback()
	return st.CounterWithRate(RateArgs{
		Name: name,
		Rate: st.counterSampleRate,
	})
}

// CounterWithRate returns a counter metrics to the name,
// with sample rate passed in instead of inherited from StatsdConfig.
func (st *Statsd) CounterWithRate(args RateArgs) metrics.Counter {
	st = st.fallback()
	counter := st.statsd.NewCounter(args.Name, args.ReportingRate())
	if args.Rate >= 1 {
		return counter
	}
	return SampledCounter{
		Counter: counter,
		Rate:    args.Rate,
	}
}

// Histogram returns a histogram metrics to the name with no specific unit,
// with sample rate inherited from StatsdConfig.
func (st *Statsd) Histogram(name string) metrics.Histogram {
	st = st.fallback()
	return st.HistogramWithRate(RateArgs{
		Name: name,
		Rate: st.histogramSampleRate,
	})
}

// HistogramWithRate returns a histogram metrics to the name with no specific
// unit, with sample rate passed in instead of inherited from StatsdConfig.
func (st *Statsd) HistogramWithRate(args RateArgs) metrics.Histogram {
	st = st.fallback()
	histogram := st.statsd.NewHistogram(args.Name, args.ReportingRate())
	if args.Rate >= 1 {
		return histogram
	}
	return SampledHistogram{
		Histogram: histogram,
		Rate:      args.Rate,
	}
}

// Timing returns a histogram metrics to the name with milliseconds as the unit,
// with sample rate inherited from StatsdConfig.
func (st *Statsd) Timing(name string) metrics.Histogram {
	st = st.fallback()
	return st.TimingWithRate(RateArgs{
		Name: name,
		Rate: st.histogramSampleRate,
	})
}

// TimingWithRate returns a histogram metrics to the name with milliseconds as
// the unit, with sample rate passed in instead of inherited from StatsdConfig.
func (st *Statsd) TimingWithRate(args RateArgs) metrics.Histogram {
	st = st.fallback()
	histogram := st.statsd.NewTiming(args.Name, args.ReportingRate())
	if args.Rate >= 1 {
		return histogram
	}
	return SampledHistogram{
		Histogram: histogram,
		Rate:      args.Rate,
	}
}

// Gauge returns a gauge metrics to the name.
//
// Please note that gauges are considered "low level".
// In most cases when you use a Gauge, you want to use RuntimeGauge instead.
func (st *Statsd) Gauge(name string) metrics.Gauge {
	st = st.fallback()
	return st.statsd.NewGauge(name)
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

// Close flushes all metrics not written to collector (if Address was set),
// and cancel the context,
// thus stop all background goroutines started by this Statsd.
//
// After Close() is called,
// no more metrics will be send to the remote collector,
// similar to the situation that this Statsd was initialized without Address
// set,
// but the difference is that calling Close() again will do the manual flush
// again.
//
// After Close() is called,
// Ctx() will always return an already canceled context.
//
// This function is useful for jobs that exit,
// to make sure that all metrics are flushed before exiting.
// For server code, there's usually no need to call Close(),
// just cancel the context object passed in is sufficient.
// But server code can also choose to pass in a background context,
// and use Close() call to do the cleanup instead of canceling the context.
func (st *Statsd) Close() error {
	st.cancel()
	if st.writer == nil {
		return nil
	}
	return st.writer.doWrite(st.statsd, log.KitLogger(st.cfg.LogLevel))
}

// WriteTo calls the underlying statsd implementation's WriteTo function.
//
// Doing this will flush all the buffered metrics to the writer,
// so in most cases you shouldn't be using it in production code.
// But it's useful in unit tests to verify that you have the correct metrics you
// want to report.
func (st *Statsd) WriteTo(w io.Writer) (n int64, err error) {
	return st.fallback().statsd.WriteTo(w)
}

func (st *Statsd) incActiveRequests() {
	st = st.fallback()
	atomic.AddInt64(&st.activeRequests, 1)
}

func (st *Statsd) decActiveRequests() {
	st = st.fallback()
	atomic.AddInt64(&st.activeRequests, -1)
}

func (st *Statsd) getActiveRequests() int64 {
	st = st.fallback()
	return atomic.LoadInt64(&st.activeRequests)
}
