package httpbp

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/reddit/baseplate.go/internal/prometheusbpint"
	"github.com/reddit/baseplate.go/prometheusbp"
)

const (
	methodLabel     = "http_method"
	successLabel    = "http_success"
	codeLabel       = "http_response_code"
	clientNameLabel = "http_client_name"
	endpointLabel   = "http_endpoint"
)

var (
	serverLabels = []string{
		methodLabel,
		successLabel,
		endpointLabel,
	}

	serverLatency = promauto.With(prometheusbpint.GlobalRegistry).NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_server_latency_seconds",
		Help:    "HTTP server request latencies",
		Buckets: prometheusbp.DefaultLatencyBuckets,
	}, serverLabels)

	serverRequestSize = promauto.With(prometheusbpint.GlobalRegistry).NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_server_request_size_bytes",
		Help:    "Request size",
		Buckets: prometheusbp.DefaultLatencyBuckets,
	}, serverLabels)

	serverResponseSize = promauto.With(prometheusbpint.GlobalRegistry).NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_server_response_size_bytes",
		Help:    "Response size",
		Buckets: prometheusbp.DefaultLatencyBuckets,
	}, serverLabels)

	serverTimeToWriteHeader = promauto.With(prometheusbpint.GlobalRegistry).NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_server_time_to_write_header_seconds",
		Help:    "Request size",
		Buckets: prometheusbp.DefaultLatencyBuckets,
	}, serverLabels)

	serverTimeToFirstByte = promauto.With(prometheusbpint.GlobalRegistry).NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_server_time_to_first_byte_seconds",
		Help:    "Response size",
		Buckets: prometheusbp.DefaultLatencyBuckets,
	}, serverLabels)

	serverTotalRequestLabels = []string{
		methodLabel,
		successLabel,
		codeLabel,
		endpointLabel,
	}

	serverTotalRequests = promauto.With(prometheusbpint.GlobalRegistry).NewCounterVec(prometheus.CounterOpts{
		Name: "http_server_requests_total",
		Help: "Total request count",
	}, serverTotalRequestLabels)

	serverActiveRequestsLabels = []string{
		methodLabel,
		endpointLabel,
	}

	serverActiveRequests = promauto.With(prometheusbpint.GlobalRegistry).NewGaugeVec(prometheus.GaugeOpts{
		Name: "http_server_active_requests",
		Help: "The number of in-flight requests being handled by the service",
	}, serverActiveRequestsLabels)
)

var (
	clientLatencyLabels = []string{
		methodLabel,
		successLabel,
		clientNameLabel,
	}

	clientLatencyDistribution = promauto.With(prometheusbpint.GlobalRegistry).NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_client_latency_seconds",
		Help:    "HTTP client request latencies",
		Buckets: prometheusbp.DefaultLatencyBuckets,
	}, clientLatencyLabels)

	clientTotalRequestLabels = []string{
		methodLabel,
		successLabel,
		codeLabel,
		clientNameLabel,
	}

	clientTotalRequests = promauto.With(prometheusbpint.GlobalRegistry).NewCounterVec(prometheus.CounterOpts{
		Name: "http_client_requests_total",
		Help: "Total request count",
	}, clientTotalRequestLabels)

	clientActiveRequestsLabels = []string{
		methodLabel,
		clientNameLabel,
	}

	clientActiveRequests = promauto.With(prometheusbpint.GlobalRegistry).NewGaugeVec(prometheus.GaugeOpts{
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

	panicRecoverCounter = promauto.With(prometheusbpint.GlobalRegistry).NewCounterVec(prometheus.CounterOpts{
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
