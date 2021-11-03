package prometheusbp

import (
	"github.com/prometheus/client_golang/prometheus"
)

var DefaultBuckets = prometheus.ExponentialBuckets(0.0001, 1.5, 26) // 100us ~ 2.5s
