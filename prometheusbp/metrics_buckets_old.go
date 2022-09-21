//go:build !metrics_spec
// +build !metrics_spec

package prometheusbp

import (
	"github.com/prometheus/client_golang/prometheus"
)

// DefaultBuckets is the default bucket values for a prometheus histogram metric.
var DefaultBuckets = prometheus.ExponentialBuckets(0.0001, 2.5, 14) // 100us ~ 14.9s

// KafkaBuckets is the set of bucket values for Kafka-related prometheus histogram metrics.
var KafkaBuckets = prometheus.ExponentialBucketsRange(1e-4, 10, 10) // 100us - 10s
