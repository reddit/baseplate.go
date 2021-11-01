package grpcbp

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/reddit/baseplate.go/prometheusbp"
)

const (
	serviceLabel = "grpc_service"
	methodLabel  = "grpc_method"
	typeLabel    = "grpc_type"
	successLabel = "grpc_success"
	codeLabel    = "grpc_code"
	slugLabel    = "grpc_slug"
)

const (
	unary        = "unary"
	clientStream = "client_stream"
	serverStream = "server_stream"
)

var (
	serverLatencyLabels = []string{
		serviceLabel,
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
		serviceLabel,
		methodLabel,
		typeLabel,
		successLabel,
		codeLabel,
	}

	serverRequestCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "grpc_server_requests_total",
		Help: "Total RPC request count",
	}, serverRequestLabels)

	serverActiveRequestsLabels = []string{
		serviceLabel,
		methodLabel,
	}

	serverActiveRequests = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "grpc_server_active_requests",
		Help: "The number of requests being handled by the server.",
	}, serverActiveRequestsLabels)
)

var (
	clientLatencyLabels = []string{
		serviceLabel,
		methodLabel,
		typeLabel,
		successLabel,
		slugLabel,
	}

	clientLatencyDistribution = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "grpc_client_latency_seconds",
		Help:    "RPC latencies",
		Buckets: prometheusbp.DefaultBuckets,
	}, clientLatencyLabels)

	clientRequestLabels = []string{
		serviceLabel,
		methodLabel,
		typeLabel,
		successLabel,
		codeLabel,
		slugLabel,
	}

	clientRequestCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "grpc_client_requests_total",
		Help: "Total RPC request count",
	}, clientRequestLabels)

	clientActiveRequestsLabels = []string{
		serviceLabel,
		methodLabel,
		slugLabel,
	}

	clientActiveRequests = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "grpc_client_active_requests",
		Help: "The number of requests in-flight by the client.",
	}, clientActiveRequestsLabels)
)
