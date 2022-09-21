//go:build metrics_spec
// +build metrics_spec

package prometheusbp

// DefaultBuckets is the default bucket values for a prometheus histogram metric.
// These match the spec defined at https://github.snooguts.net/reddit/baseplate.spec/blob/master/component-apis/prom-metrics.md#latency-histograms
var DefaultBuckets = []float64{
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
	15.000_000, // 15s (fastly timeout)
	30.000_000, // 30s
}

// KafkaBuckets is the set of bucket values for Kafka-related prometheus histogram metrics.
// These match the spec defined at https://github.snooguts.net/reddit/baseplate.spec/blob/master/component-apis/prom-metrics.md#latency-histograms
var KafkaBuckets = DefaultBuckets
