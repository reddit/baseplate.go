package httpbp

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/reddit/baseplate.go/prometheusbp"
)

const (
	methodLabel            = "http_method"
	successLabel           = "http_success"
	endpointLabel          = "http_endpoint"
	codeLabel              = "http_response_code"
	remoteServiceSlugLabel = "http_slug"
)

var (
	serverLabels = []string{
		methodLabel,
		successLabel,
		endpointLabel,
	}

	serverLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_server_latency_seconds",
		Help:    "RPC latencies",
		Buckets: prometheusbp.DefaultBuckets,
	}, serverLabels)

	serverRequestSize = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_server_request_size_bytes",
		Help:    "Request size",
		Buckets: prometheusbp.DefaultBuckets,
	}, serverLabels)

	serverResponseSize = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_server_response_size_bytes",
		Help:    "Response size",
		Buckets: prometheusbp.DefaultBuckets,
	}, serverLabels)

	serverTimeToWriteHeader = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_server_time_to_write_header_seconds",
		Help:    "Request size",
		Buckets: prometheusbp.DefaultBuckets,
	}, serverLabels)

	serverTimeToFirstByte = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_server_time_to_first_byte_seconds",
		Help:    "Response size",
		Buckets: prometheusbp.DefaultBuckets,
	}, serverLabels)

	serverRequestLabels = []string{
		methodLabel,
		successLabel,
		endpointLabel,
		codeLabel,
	}

	serverTotalRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_server_requests_total",
		Help: "Total request count",
	}, serverRequestLabels)

	serverActiveRequestsLabels = []string{
		methodLabel,
		endpointLabel,
	}

	serverActiveRequests = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "http_server_active_requests",
		Help: "The number of in-flight requests being handled by the service",
	}, serverActiveRequestsLabels)
)

var (
	clientLatencyLabels = []string{
		methodLabel,
		successLabel,
		endpointLabel,
		remoteServiceSlugLabel,
	}

	clientLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_client_latency_seconds",
		Help:    "HTTP latencies",
		Buckets: prometheusbp.DefaultBuckets,
	}, clientLatencyLabels)

	clientRequestLabels = []string{
		methodLabel,
		successLabel,
		endpointLabel,
		codeLabel,
		remoteServiceSlugLabel,
	}

	clientTotalRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_client_requests_total",
		Help: "Total request count",
	}, clientRequestLabels)

	clientActiveRequestsLabels = []string{
		methodLabel,
		endpointLabel,
		remoteServiceSlugLabel,
	}

	clientActiveRequests = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "http_client_active_requests",
		Help: "The number of in-flight requests",
	}, clientActiveRequestsLabels)
)
