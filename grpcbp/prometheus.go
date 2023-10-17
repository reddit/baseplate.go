package grpcbp

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/reddit/baseplate.go/internal/prometheusbpint"
	"github.com/reddit/baseplate.go/prometheusbp"
)

const (
	serviceLabel    = "grpc_service"
	methodLabel     = "grpc_method"
	typeLabel       = "grpc_type"
	successLabel    = "grpc_success"
	codeLabel       = "grpc_code"
	clientNameLabel = "grpc_client_name"
)

const (
	unary        = "unary"
	bidiStream   = "bidi_stream"
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

	serverLatencyDistribution = promauto.With(prometheusbpint.GlobalRegistry).NewHistogramVec(prometheus.HistogramOpts{
		Name:    "grpc_server_latency_seconds",
		Help:    "RPC latencies",
		Buckets: prometheusbp.DefaultLatencyBuckets,
	}, serverLatencyLabels)

	serverTotalRequestLabels = []string{
		serviceLabel,
		methodLabel,
		typeLabel,
		successLabel,
		codeLabel,
	}

	serverTotalRequests = promauto.With(prometheusbpint.GlobalRegistry).NewCounterVec(prometheus.CounterOpts{
		Name: "grpc_server_requests_total",
		Help: "Total RPC request count",
	}, serverTotalRequestLabels)

	serverActiveRequestsLabels = []string{
		serviceLabel,
		methodLabel,
		typeLabel,
	}

	serverActiveRequests = promauto.With(prometheusbpint.GlobalRegistry).NewGaugeVec(prometheus.GaugeOpts{
		Name: "grpc_server_active_requests",
		Help: "The number of in-flight requests being handled by the service",
	}, serverActiveRequestsLabels)
)

var (
	clientLatencyLabels = []string{
		serviceLabel,
		methodLabel,
		typeLabel,
		successLabel,
		clientNameLabel,
	}

	clientLatencyDistribution = promauto.With(prometheusbpint.GlobalRegistry).NewHistogramVec(prometheus.HistogramOpts{
		Name:    "grpc_client_latency_seconds",
		Help:    "RPC latencies",
		Buckets: prometheusbp.DefaultLatencyBuckets,
	}, clientLatencyLabels)

	clientTotalRequestLabels = []string{
		serviceLabel,
		methodLabel,
		typeLabel,
		successLabel,
		codeLabel,
		clientNameLabel,
	}

	clientTotalRequests = promauto.With(prometheusbpint.GlobalRegistry).NewCounterVec(prometheus.CounterOpts{
		Name: "grpc_client_requests_total",
		Help: "Total RPC request count",
	}, clientTotalRequestLabels)

	clientActiveRequestsLabels = []string{
		serviceLabel,
		methodLabel,
		typeLabel,
		clientNameLabel,
	}

	clientActiveRequests = promauto.With(prometheusbpint.GlobalRegistry).NewGaugeVec(prometheus.GaugeOpts{
		Name: "grpc_client_active_requests",
		Help: "The number of in-flight requests",
	}, clientActiveRequestsLabels)
)

// serviceAndMethodSlug splits the UnaryServerInfo.FullMethod and returns
// the package.service part separate from the method part.
// ref: https://pkg.go.dev/google.golang.org/grpc#UnaryServerInfo
func serviceAndMethodSlug(fullMethod string) (service, method string) {
	fullMethod = strings.TrimPrefix(fullMethod, "/")
	split := strings.SplitN(fullMethod, "/", 2)
	if len(split) < 2 {
		return "", fullMethod
	}
	return split[0], split[1]
}
