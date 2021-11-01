package thriftbp

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	serviceLabel             = "thrift_service"
	methodLabel              = "thrift_method"
	successLabel             = "thrift_success"
	exceptionLabel           = "thrift_exception_type"
	baseplateStatusLabel     = "thrift_baseplate_status"
	baseplateStatusCodeLabel = "thrift_baseplate_status_code"
)

var (
	thriftLatencyLabels = []string{
		serviceLabel,
		methodLabel,
		successLabel,
	}

	latencyDistribution = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "thrift_server_latency_seconds",
		Help:    "RPC latencies",
		Buckets: prometheus.ExponentialBuckets(0.0001, 1.5, 26), // 100us ~ 2.5s
	}, thriftLatencyLabels)

	thriftRequestLabels = []string{
		serviceLabel,
		methodLabel,
		successLabel,
		exceptionLabel,
		baseplateStatusLabel,
		baseplateStatusCodeLabel,
	}

	rpcRequestCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "thrift_server_requests_total",
		Help: "Total RPC request count",
	}, thriftRequestLabels)

	activeRequestsLabels = []string{
		serviceLabel,
		methodLabel,
	}

	activeRequests = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "thrift_server_active_requests",
		Help: "The number of in-flight requests being handled by the service.",
	}, activeRequestsLabels)
)
