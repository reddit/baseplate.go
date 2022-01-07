package thriftbp

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/reddit/baseplate.go/prometheusbp"
)

const (
	methodLabel              = "thrift_method"
	successLabel             = "thrift_success"
	exceptionLabel           = "thrift_exception_type"
	baseplateStatusLabel     = "thrift_baseplate_status"
	baseplateStatusCodeLabel = "thrift_baseplate_status_code"
	remoteServiceSlugLabel   = "thrift_slug"
)

var (
	serverLatencyLabels = []string{
		methodLabel,
		successLabel,
	}

	serverLatencyDistribution = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "thrift_server_latency_seconds",
		Help:    "RPC latencies",
		Buckets: prometheusbp.DefaultBuckets,
	}, serverLatencyLabels)

	serverRequestLabels = []string{
		methodLabel,
		successLabel,
		exceptionLabel,
		baseplateStatusLabel,
		baseplateStatusCodeLabel,
	}

	serverRPCRequestCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "thrift_server_requests_total",
		Help: "Total RPC request count",
	}, serverRequestLabels)

	serverActiveRequestsLabels = []string{
		methodLabel,
	}

	serverActiveRequests = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "thrift_server_active_requests",
		Help: "The number of in-flight requests being handled by the service",
	}, serverActiveRequestsLabels)
)

var (
	clientLatencyLabels = []string{
		methodLabel,
		successLabel,
		remoteServiceSlugLabel,
	}

	clientLatencyDistribution = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "thrift_client_latency_seconds",
		Help:    "RPC latencies",
		Buckets: prometheusbp.DefaultBuckets,
	}, clientLatencyLabels)

	clientRequestLabels = []string{
		methodLabel,
		successLabel,
		exceptionLabel,
		baseplateStatusLabel,
		baseplateStatusCodeLabel,
		remoteServiceSlugLabel,
	}

	clientRPCRequestCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "thrift_client_requests_total",
		Help: "Total RPC request count",
	}, clientRequestLabels)

	clientActiveRequestsLabels = []string{
		methodLabel,
		remoteServiceSlugLabel,
	}

	clientActiveRequests = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "thrift_client_active_requests",
		Help: "The number of in-flight requests",
	}, clientActiveRequestsLabels)
)
