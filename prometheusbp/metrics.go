package prometheusbp

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

	// Deprecated: Please use DefaultLatencyBuckets instead for latencies, and define your own buckets for non-latency histograms
	DefaultBuckets = DefaultLatencyBuckets
)

// BoolString returns the string version of a boolean value that should be used
// in a prometheus metric label.
func BoolString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
