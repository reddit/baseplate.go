package thriftbp

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	thriftLabels = []string{
		"thrift_service",
		"thrift_method",
		"thrift_success",
		"thrift_exception_type",
		"thrift_baseplate_status",
		"thrift_baseplate_status_code",
	}

	latencyDistribution = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "thrift_server_handling_seconds",
		Help:    "RPC latencies",
		Buckets: prometheus.ExponentialBuckets(0.0001, 1.5, 26), // 100us ~ 2.5s
	}, thriftLabels)

	rpcStatusCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "thrift_server_handled_total",
		Help: "Total RPC request count",
	}, thriftLabels)

	activeRequests = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "thrift_active_requests",
		Help: "The number of requests being handled by the service.",
	}, thriftLabels)
)

// SetPrometheusServiceLabels adds Prometheus labels that are specific to the Thrift service.
// The Thrift service name is the fully qualified name, for example: <project>.<serviceName>
func SetPrometheusServiceLabels(thriftServiceName string) {
	latencyDistribution.With(prometheus.Labels{
		"thrift_service": thriftServiceName,
	})
	rpcStatusCounter.With(prometheus.Labels{
		"thrift_service": thriftServiceName,
	})
	activeRequests.With(prometheus.Labels{
		"thrift_service": thriftServiceName,
	})
}
