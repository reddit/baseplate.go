package grpcbp

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/reddit/baseplate.go/prometheusbp"
)

const (
	localServiceLabel      = "grpc_service"
	methodLabel            = "grpc_method"
	typeLabel              = "grpc_type"
	successLabel           = "grpc_success"
	codeLabel              = "grpc_code"
	remoteServiceSlugLabel = "grpc_slug"
)

const (
	unary        = "unary"
	clientStream = "client_stream"
	serverStream = "server_stream"
)

var (
	serverLatencyLabels = []string{
		localServiceLabel,
		methodLabel,
		typeLabel,
		successLabel,
	}

	serverLatencyDistribution = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "grpc_server_latency_seconds",
		Help:    "RPC latencies",
		Buckets: prometheusbp.DefaultBuckets,
	}, serverLatencyLabels)

	serverRequestLabels = []string{
		localServiceLabel,
		methodLabel,
		typeLabel,
		successLabel,
		codeLabel,
	}

	serverRPCRequestCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "grpc_server_requests_total",
		Help: "Total RPC request count",
	}, serverRequestLabels)

	serverActiveRequestsLabels = []string{
		localServiceLabel,
		methodLabel,
	}

	serverActiveRequests = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "grpc_server_active_requests",
		Help: "The number of in-flight requests being handled by the service",
	}, serverActiveRequestsLabels)
)

var (
	clientLatencyLabels = []string{
		localServiceLabel,
		methodLabel,
		typeLabel,
		successLabel,
		remoteServiceSlugLabel,
	}

	clientLatencyDistribution = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "grpc_client_latency_seconds",
		Help:    "RPC latencies",
		Buckets: prometheusbp.DefaultBuckets,
	}, clientLatencyLabels)

	clientRequestLabels = []string{
		localServiceLabel,
		methodLabel,
		typeLabel,
		successLabel,
		codeLabel,
		remoteServiceSlugLabel,
	}

	clientRPCRequestCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "grpc_client_requests_total",
		Help: "Total RPC request count",
	}, clientRequestLabels)

	clientActiveRequestsLabels = []string{
		localServiceLabel,
		methodLabel,
		remoteServiceSlugLabel,
	}

	clientActiveRequests = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "grpc_client_active_requests",
		Help: "The number of in-flight requests",
	}, clientActiveRequestsLabels)
)
