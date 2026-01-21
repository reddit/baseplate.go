package prometheusbp

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	// DefaultNativeHistogramBucketFactor is the default bucket factor for native histograms.
	// This controls the resolution of the histogram buckets.
	DefaultNativeHistogramBucketFactor = 1.1

	// DefaultNativeHistogramMaxBucketNumber is the default maximum number of buckets
	// for native histograms.
	DefaultNativeHistogramMaxBucketNumber = 160

	// DefaultNativeHistogramMinResetDuration is the default minimum duration between
	// resets for native histograms.
	DefaultNativeHistogramMinResetDuration = time.Hour
)

var (
	// DefaultLatencyBuckets is the default bucket values for a prometheus latency histogram metric.
	//
	// These match the spec defined at https://github.snooguts.net/reddit/baseplate.spec/blob/master/component-apis/prom-metrics.md#latency-histograms
	DefaultLatencyBuckets = []float64{
		0.000_100,  // 100us
		0.000_500,  // 500us
		0.001_000,  // 1ms
		0.002_500,  // 2.5ms
		0.005_000,  // 5ms
		0.010_000,  // 10ms
		0.025_000,  // 25ms
		0.050_000,  // 50ms
		0.100_000,  // 100ms
		0.250_000,  // 250ms
		0.500_000,  // 500ms
		1.000_000,  // 1s
		5.000_000,  // 5s
		15.000_000, // 15s
		30.000_000, // 30s
	}

	// Deprecated: Please use DefaultLatencyBuckets instead for latencies if the buckets suit your needs, otherwise define your own buckets.
	DefaultBuckets = DefaultLatencyBuckets

	DefaultNativeHistogramBucketing = &NativeHistogramBucketing{
		BucketFactor:     DefaultNativeHistogramBucketFactor,
		MaxBucketNumber:  DefaultNativeHistogramMaxBucketNumber,
		MinResetDuration: DefaultNativeHistogramMinResetDuration,
	}
)

type HistogramOpts struct {
	Name                     string
	Help                     string
	LegacyBuckets            []float64
	NativeHistogramBucketing *NativeHistogramBucketing
}

type NativeHistogramBucketing struct {
	BucketFactor     float64
	MaxBucketNumber  uint32
	MinResetDuration time.Duration
}

func (opts HistogramOpts) ToPrometheus() prometheus.HistogramOpts {
	buckets := opts.LegacyBuckets
	if buckets == nil {
		buckets = DefaultLatencyBuckets
	}

	nativeBucketing := opts.NativeHistogramBucketing
	if nativeBucketing == nil {
		nativeBucketing = DefaultNativeHistogramBucketing
	} else {
		// Backfill any native histogram configs without our defaults
		nb := *nativeBucketing // copy to avoid mutation
		if nb.BucketFactor == 0 {
			nb.BucketFactor = DefaultNativeHistogramBucketing.BucketFactor
		}
		if nb.MaxBucketNumber == 0 {
			nb.MaxBucketNumber = DefaultNativeHistogramBucketing.MaxBucketNumber
		}
		if nb.MinResetDuration == 0 {
			nb.MinResetDuration = DefaultNativeHistogramBucketing.MinResetDuration
		}
		nativeBucketing = &nb
	}

	return prometheus.HistogramOpts{
		Name:                            opts.Name,
		Help:                            opts.Help,
		Buckets:                         buckets,
		NativeHistogramBucketFactor:     nativeBucketing.BucketFactor,
		NativeHistogramMaxBucketNumber:  nativeBucketing.MaxBucketNumber,
		NativeHistogramMinResetDuration: nativeBucketing.MinResetDuration,
	}
}

// BoolString returns the string version of a boolean value that should be used
// in a prometheus metric label.
func BoolString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
