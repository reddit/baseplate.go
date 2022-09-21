//go:build !metrics_spec
// +build !metrics_spec

package prometheusbp

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// DefaultLatencyBuckets is the default bucket values for a prometheus histogram metric.
	DefaultLatencyBuckets = prometheus.ExponentialBuckets(0.0001, 2.5, 14) // 100us ~ 14.9s
	// KafkaLatencyBuckets is the set of bucket values for Kafka-related prometheus histogram metrics.
	KafkaLatencyBuckets = prometheus.ExponentialBucketsRange(1e-4, 10, 10) // 100us - 10s
	// Deprecated: Please use DefaultLatencyBuckets instead for latencies, and define your own buckets for non-latency histograms
	DefaultBuckets = DefaultLatencyBuckets
)
