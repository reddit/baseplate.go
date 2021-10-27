package prometheusbp

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

func NewLatencyDistribution(namePrefix string, labels []string) *prometheus.HistogramVec {
	return promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    fmt.Sprintf("%s_latency_seconds", namePrefix),
		Help:    "Request latencies",
		Buckets: prometheus.ExponentialBuckets(0.0001, 1.5, 26), // 100us ~ 2.5s
	}, labels)
}

func NewRPCRequest(namePrefix string, labels []string) *prometheus.CounterVec {
	return promauto.NewCounterVec(prometheus.CounterOpts{
		Name: fmt.Sprintf("%s_requests_total", namePrefix),
		Help: "Total request count",
	}, labels)
}

func NewActiveRequest(namePrefix string, labels []string) *prometheus.GaugeVec {
	return promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: fmt.Sprintf("%s_active_requests", namePrefix),
		Help: "The number of requests being handled by the server.",
	}, labels)
}
