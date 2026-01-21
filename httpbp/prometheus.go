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

var payloadSizeBuckets = []float64{
	100,       // 100 B
	500,       // 500 B
	1 << 10,   // 1 KiB
	4 << 10,   // 4 KiB
	10 << 10,  // 10 KiB
	50 << 10,  // 50 KiB
	100 << 10, // 100 KiB
	500 << 10, // 500 KiB
	1 << 20,   // 1 MiB
	5 << 20,   // 5 MiB
	10 << 20,  // 10 MiB
	50 << 20,  // 50 MiB
	100 << 20, // 100 MiB
}

var (
	serverLabels = []string{
		methodLabel,
		successLabel,
		endpointLabel,
	}

	serverLatency = promauto.With(prometheusbpint.GlobalRegistry).NewHistogramVec(prometheusbp.HistogramOpts{
		Name: "http_server_latency_seconds",
		Help: "HTTP server request latencies",
	}.ToPrometheus(), serverLabels)

	// hasHistClassic = promauto.With(prometheusbpint.GlobalRegistry).NewHistogramVec(prometheus.HistogramOpts{
	// 	Name:    "has_native_hist",
	// 	Help:    "HTTP server request latencies",
	// 	Buckets: prometheusbp.DefaultLatencyBuckets,
	// 	ConstLabels: prometheus.Labels{
	// 		"native": "false",
	// 	},
	// }, serverLabels)

	hasHistNative = promauto.With(prometheusbpint.GlobalRegistry).NewHistogramVec(prometheus.HistogramOpts{
		Name:                            "has_native_hist",
		Help:                            "HTTP server request latencies",
		NativeHistogramBucketFactor:     prometheusbp.DefaultNativeHistogramBucketFactor,
		NativeHistogramMaxBucketNumber:  prometheusbp.DefaultNativeHistogramMaxBucketNumber,
		NativeHistogramMinResetDuration: prometheusbp.DefaultNativeHistogramMinResetDuration,
		ConstLabels: prometheus.Labels{
			"native": "true",
		},
	}, serverLabels)

	serverRequestSize = promauto.With(prometheusbpint.GlobalRegistry).NewHistogramVec(prometheusbp.HistogramOpts{
		Name:          "http_server_request_size_bytes",
		Help:          "Request size",
		LegacyBuckets: payloadSizeBuckets,
	}.ToPrometheus(), serverLabels)

	serverResponseSize = promauto.With(prometheusbpint.GlobalRegistry).NewHistogramVec(prometheusbp.HistogramOpts{
		Name:          "http_server_response_size_bytes",
		Help:          "Response size",
		LegacyBuckets: payloadSizeBuckets,
	}.ToPrometheus(), serverLabels)

	serverTimeToWriteHeader = promauto.With(prometheusbpint.GlobalRegistry).NewHistogramVec(prometheusbp.HistogramOpts{
		Name: "http_server_time_to_write_header_seconds",
		Help: "Request size",
	}.ToPrometheus(), serverLabels)

	serverTimeToFirstByte = promauto.With(prometheusbpint.GlobalRegistry).NewHistogramVec(prometheusbp.HistogramOpts{
		Name: "http_server_time_to_first_byte_seconds",
		Help: "Time elapsed before first byte was sent",
	}.ToPrometheus(), serverLabels)

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
		endpointLabel,
		successLabel,
		clientNameLabel,
	}

	clientLatencyDistribution = promauto.With(prometheusbpint.GlobalRegistry).NewHistogramVec(prometheusbp.HistogramOpts{
		Name: "http_client_latency_seconds",
		Help: "HTTP client request latencies",
	}.ToPrometheus(), clientLatencyLabels)

	clientTotalRequestLabels = []string{
		methodLabel,
		endpointLabel,
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
		endpointLabel,
		clientNameLabel,
	}

	clientActiveRequests = promauto.With(prometheusbpint.GlobalRegistry).NewGaugeVec(prometheus.GaugeOpts{
		Name: "http_client_active_requests",
		Help: "The number of in-flight requests",
	}, clientActiveRequestsLabels)
)

var (
	panicRecoverLabels = []string{
		methodLabel,
	}

	panicRecoverCounter = promauto.With(prometheusbpint.GlobalRegistry).NewCounterVec(prometheus.CounterOpts{
		Name: "httpbp_server_recovered_panics_total",
		Help: "The number of panics recovered from http server handlers",
	}, panicRecoverLabels)
)

// PerformanceMonitoringMiddleware returns optional Prometheus historgram metrics for monitoring the following:
//  1. http server time to write header in seconds
//  2. http server time to write header in seconds
func PerformanceMonitoringMiddleware() (timeToWriteHeader, timeToFirstByte *prometheus.HistogramVec) {
	return serverTimeToWriteHeader, serverTimeToFirstByte
}
