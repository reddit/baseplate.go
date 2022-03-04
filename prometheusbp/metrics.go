package prometheusbp

import (
	"github.com/prometheus/client_golang/prometheus"
)

// DefaultBuckets is the default bucket values for a prometheus histogram metric.
var DefaultBuckets = prometheus.ExponentialBuckets(0.0001, 2.5, 14) // 100us ~ 14.9s

// BoolString returns the string version of a boolean value that should be used
// in a prometheus metric label.
func BoolString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
