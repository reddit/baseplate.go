package thriftbp

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/reddit/baseplate.go/prometheusbp"
)

const (
	serviceLabel             = "thrift_service"
	methodLabel              = "thrift_method"
	successLabel             = "thrift_success"
	exceptionLabel           = "thrift_exception_type"
	baseplateStatusLabel     = "thrift_baseplate_status"
	baseplateStatusCodeLabel = "thrift_baseplate_status_code"
	slugLabel                = "thrift_slug"
)

var (
	serverThriftLatencyLabels = []string{
		serviceLabel,
		methodLabel,
		successLabel,
	}

	serverLatencyDistribution = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "thrift_server_latency_seconds",
		Help:    "RPC latencies",
		Buckets: prometheusbp.DefaultBuckets,
	}, serverThriftLatencyLabels)

	serverThriftRequestLabels = []string{
		serviceLabel,
		methodLabel,
		successLabel,
		exceptionLabel,
		baseplateStatusLabel,
		baseplateStatusCodeLabel,
	}

	serverRPCRequestCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "thrift_server_requests_total",
		Help: "Total RPC request count",
	}, serverThriftRequestLabels)

	serverActiveRequestsLabels = []string{
		serviceLabel,
		methodLabel,
	}

	serverActiveRequests = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "thrift_server_active_requests",
		Help: "The number of in-flight requests being handled by the service",
	}, serverActiveRequestsLabels)
)

var (
	clientThriftLatencyLabels = []string{
		serviceLabel,
		methodLabel,
		successLabel,
		slugLabel,
	}

	clientLatencyDistribution = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "thrift_client_latency_seconds",
		Help:    "RPC latencies",
		Buckets: prometheusbp.DefaultBuckets,
	}, clientThriftLatencyLabels)

	clientThriftRequestLabels = []string{
		serviceLabel,
		methodLabel,
		successLabel,
		exceptionLabel,
		baseplateStatusLabel,
		baseplateStatusCodeLabel,
		slugLabel,
	}

	clientRPCRequestCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "thrift_client_requests_total",
		Help: "Total RPC request count",
	}, clientThriftRequestLabels)

	clientActiveRequestsLabels = []string{
		serviceLabel,
		methodLabel,
	}

	clientActiveRequests = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "thrift_client_active_requests",
		Help: "The number of in-flight requests",
	}, clientActiveRequestsLabels)
)
