package metricsbp

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
)

var activeRequests = prometheus.NewGauge(prometheus.GaugeOpts{
	Name: "active_requests",
	Help: "The number of requests being handled by the service.",
})

type PrometheusMetrics struct {
}

// NewPrometheusMetrics creates a new PrometheusMetrics object that is used to register new metrics for monitoring.
func NewPrometheusMetrics(ctx context.Context, cfg Config) *PrometheusMetrics {
	prometheus.MustRegister(activeRequests)
	return &PrometheusMetrics{}
}

func (pm *PrometheusMetrics) incActiveRequests() {
	activeRequests.Inc()
}

func (pm *PrometheusMetrics) decActiveRequests() {
	activeRequests.Dec()
}
