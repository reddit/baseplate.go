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
	ThriftLabels = []string{
		serviceLabel,
		methodLabel,
		successLabel,
		exceptionLabel,
		baseplateStatusLabel,
		baseplateStatusCodeLabel,
	}

	LatencyDistribution = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "thrift_server_handling_seconds",
		Help:    "RPC latencies",
		Buckets: prometheus.ExponentialBuckets(0.0001, 1.5, 26), // 100us ~ 2.5s
	}, ThriftLabels)

	RPCStatusCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "thrift_server_handled",
		Help: "Total RPC request count",
	}, ThriftLabels)

	ActiveRequestsLabels = []string{
		serviceLabel,
		methodLabel,
	}

	ActiveRequests = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "thrift_active_requests",
		Help: "The number of requests being handled by the service.",
	}, ActiveRequestsLabels)
)
