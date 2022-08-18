package httpbp

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	//lint:ignore SA1019 This library is internal only, not actually deprecated
	"github.com/reddit/baseplate.go/internalv2compat"
	"github.com/reddit/baseplate.go/prometheusbp"
)

const (
	methodLabel     = "http_method"
	successLabel    = "http_success"
	codeLabel       = "http_response_code"
	serverSlugLabel = "http_client_name"
	endpointLabel   = "http_endpoint"
)

var (
	serverLabels = []string{
		methodLabel,
		successLabel,
		endpointLabel,
	}

	serverLatency = promauto.With(internalv2compat.GlobalRegistry).NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_server_latency_seconds",
		Help:    "HTTP server request latencies",
		Buckets: prometheusbp.DefaultBuckets,
	}, serverLabels)

	serverRequestSize = promauto.With(internalv2compat.GlobalRegistry).NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_server_request_size_bytes",
		Help:    "Request size",
		Buckets: prometheusbp.DefaultBuckets,
	}, serverLabels)

	serverResponseSize = promauto.With(internalv2compat.GlobalRegistry).NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_server_response_size_bytes",
		Help:    "Response size",
		Buckets: prometheusbp.DefaultBuckets,
	}, serverLabels)

	serverTimeToWriteHeader = promauto.With(internalv2compat.GlobalRegistry).NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_server_time_to_write_header_seconds",
		Help:    "Request size",
		Buckets: prometheusbp.DefaultBuckets,
	}, serverLabels)

	serverTimeToFirstByte = promauto.With(internalv2compat.GlobalRegistry).NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_server_time_to_first_byte_seconds",
		Help:    "Response size",
		Buckets: prometheusbp.DefaultBuckets,
	}, serverLabels)

	serverTotalRequestLabels = []string{
		methodLabel,
		successLabel,
		codeLabel,
		endpointLabel,
	}

	serverTotalRequests = promauto.With(internalv2compat.GlobalRegistry).NewCounterVec(prometheus.CounterOpts{
		Name: "http_server_requests_total",
		Help: "Total request count",
	}, serverTotalRequestLabels)

	serverActiveRequestsLabels = []string{
		methodLabel,
		endpointLabel,
	}

	serverActiveRequests = promauto.With(internalv2compat.GlobalRegistry).NewGaugeVec(prometheus.GaugeOpts{
		Name: "http_server_active_requests",
		Help: "The number of in-flight requests being handled by the service",
	}, serverActiveRequestsLabels)
)

var (
	clientLatencyLabels = []string{
		methodLabel,
		successLabel,
		serverSlugLabel,
	}

	clientLatencyDistribution = promauto.With(internalv2compat.GlobalRegistry).NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_client_latency_seconds",
		Help:    "HTTP client request latencies",
		Buckets: prometheusbp.DefaultBuckets,
	}, clientLatencyLabels)

	clientTotalRequestLabels = []string{
		methodLabel,
		successLabel,
		codeLabel,
		serverSlugLabel,
	}

	clientTotalRequests = promauto.With(internalv2compat.GlobalRegistry).NewCounterVec(prometheus.CounterOpts{
		Name: "http_client_requests_total",
		Help: "Total request count",
	}, clientTotalRequestLabels)

	clientActiveRequestsLabels = []string{
		methodLabel,
		serverSlugLabel,
	}

	clientActiveRequests = promauto.With(internalv2compat.GlobalRegistry).NewGaugeVec(prometheus.GaugeOpts{
		Name: "http_client_active_requests",
		Help: "The number of in-flight requests",
	}, clientActiveRequestsLabels)
)

const (
	// Note that this is not used by prometheus metrics defined in Baseplate spec.
	promNamespace   = "httpbp"
	subsystemServer = "server"
)

var (
	panicRecoverLabels = []string{
		methodLabel,
	}

	panicRecoverCounter = promauto.With(internalv2compat.GlobalRegistry).NewCounterVec(prometheus.CounterOpts{
		Namespace: promNamespace,
		Subsystem: subsystemServer,
		Name:      "panic_recover_total",
		Help:      "The number of panics recovered from http server handlers",
	}, panicRecoverLabels)
)

// PerformanceMonitoringMiddleware returns optional Prometheus historgram metrics for monitoring the following:
//  1. http server time to write header in seconds
//  2. http server time to write header in seconds
func PerformanceMonitoringMiddleware() (timeToWriteHeader, timeToFirstByte *prometheus.HistogramVec) {
	return serverTimeToWriteHeader, serverTimeToFirstByte
}
