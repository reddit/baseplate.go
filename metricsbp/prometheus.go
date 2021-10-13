package metricsbp

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var activeRequests = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "active_requests",
	Help: "The number of requests being handled by the service.",
})

type PrometheusMetrics struct {
	ActiveRequests prometheus.Gauge
}

// NewPrometheusMetrics creates a new PrometheusMetrics object that is used to register new metrics for monitoring.
func NewPrometheusMetrics(ctx context.Context, cfg Config) *PrometheusMetrics {
	return &PrometheusMetrics{
		ActiveRequests: activeRequests,
	}
}
