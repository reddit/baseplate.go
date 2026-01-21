package thriftbp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/reddit/baseplate.go/clientpool"
	"github.com/reddit/baseplate.go/internal/prometheusbpint"
	"github.com/reddit/baseplate.go/prometheusbp"
)

const (
	methodLabel              = "thrift_method"
	successLabel             = "thrift_success"
	exceptionLabel           = "thrift_exception_type"
	baseplateStatusLabel     = "thrift_baseplate_status"
	baseplateStatusCodeLabel = "thrift_baseplate_status_code"
	clientNameLabel          = "thrift_client_name"
)

var (
	serverLatencyLabels = []string{
		methodLabel,
		successLabel,
	}

	serverLatencyDistribution = promauto.With(prometheusbpint.GlobalRegistry).NewHistogramVec(prometheusbp.HistogramOpts{
		Name: "thrift_server_latency_seconds",
		Help: "RPC latencies",
	}.ToPrometheus(), serverLatencyLabels)

	serverTotalRequestLabels = []string{
		methodLabel,
		successLabel,
		exceptionLabel,
		baseplateStatusLabel,
		baseplateStatusCodeLabel,
	}

	serverTotalRequests = promauto.With(prometheusbpint.GlobalRegistry).NewCounterVec(prometheus.CounterOpts{
		Name: "thrift_server_requests_total",
		Help: "Total RPC request count",
	}, serverTotalRequestLabels)

	serverActiveRequestsLabels = []string{
		methodLabel,
	}

	serverActiveRequests = promauto.With(prometheusbpint.GlobalRegistry).NewGaugeVec(prometheus.GaugeOpts{
		Name: "thrift_server_active_requests",
		Help: "The number of in-flight requests being handled by the service",
	}, serverActiveRequestsLabels)
)

var (
	clientLatencyLabels = []string{
		methodLabel,
		successLabel,
		clientNameLabel,
	}

	clientLatencyDistribution = promauto.With(prometheusbpint.GlobalRegistry).NewHistogramVec(prometheusbp.HistogramOpts{
		Name: "thrift_client_latency_seconds",
		Help: "RPC latencies",
	}.ToPrometheus(), clientLatencyLabels)

	clientTotalRequestLabels = []string{
		methodLabel,
		successLabel,
		exceptionLabel,
		baseplateStatusLabel,
		baseplateStatusCodeLabel,
		clientNameLabel,
	}

	clientTotalRequests = promauto.With(prometheusbpint.GlobalRegistry).NewCounterVec(prometheus.CounterOpts{
		Name: "thrift_client_requests_total",
		Help: "Total RPC request count",
	}, clientTotalRequestLabels)

	clientActiveRequestsLabels = []string{
		methodLabel,
		clientNameLabel,
	}

	clientActiveRequests = promauto.With(prometheusbpint.GlobalRegistry).NewGaugeVec(prometheus.GaugeOpts{
		Name: "thrift_client_active_requests",
		Help: "The number of in-flight requests",
	}, clientActiveRequestsLabels)
)

var (
	serverConnectionsGauge = promauto.With(prometheusbpint.GlobalRegistry).NewGauge(prometheus.GaugeOpts{
		Name: "thriftbp_server_connections",
		Help: "The number of client connections established to the service",
	})
)

var (
	ttlClientReplaceLabels = []string{
		clientNameLabel,
		successLabel,
	}

	ttlClientReplaceCounter = promauto.With(prometheusbpint.GlobalRegistry).NewCounterVec(prometheus.CounterOpts{
		Name: "thriftbp_ttl_client_connection_housekeeping_total",
		Help: "Total connection housekeeping (replacing the connection in the background) done in thrift ttlClient",
	}, ttlClientReplaceLabels)
)

const (
	protoLabel = "thrift_proto"
)

var (
	serverPayloadSizeLabels = []string{
		methodLabel,
		protoLabel,
	}

	clientPayloadSizeLabels = []string{
		methodLabel,
		clientNameLabel,
		successLabel,
	}

	// 8 bytes to 4 mebibytes
	// some endpoints can have very small, almost zero payloads (for example,
	// is_healthy), but we do have some endpoints with very large payloads
	// (up to ~500 KiB).
	payloadSizeBuckets = prometheus.ExponentialBuckets(8, 2, 20)

	serverPayloadSizeRequestBytes = promauto.With(prometheusbpint.GlobalRegistry).NewHistogramVec(prometheusbp.HistogramOpts{
		Name:          "thriftbp_server_request_payload_size_bytes",
		Help:          "The (approximate) size of thrift request payloads",
		LegacyBuckets: payloadSizeBuckets,
	}.ToPrometheus(), serverPayloadSizeLabels)

	serverPayloadSizeResponseBytes = promauto.With(prometheusbpint.GlobalRegistry).NewHistogramVec(prometheusbp.HistogramOpts{
		Name:          "thriftbp_server_response_payload_size_bytes",
		Help:          "The (approximate) size of thrift response payloads",
		LegacyBuckets: payloadSizeBuckets,
	}.ToPrometheus(), serverPayloadSizeLabels)

	clientPayloadSizeRequestBytes = promauto.With(prometheusbpint.GlobalRegistry).NewHistogramVec(prometheusbp.HistogramOpts{
		Name:          "thriftbp_client_request_payload_size_bytes",
		Help:          "The size of thrift request payloads",
		LegacyBuckets: payloadSizeBuckets,
	}.ToPrometheus(), clientPayloadSizeLabels)

	clientPayloadSizeResponseBytes = promauto.With(prometheusbpint.GlobalRegistry).NewHistogramVec(prometheusbp.HistogramOpts{
		Name:          "thriftbp_client_response_payload_size_bytes",
		Help:          "The size of thrift response payloads",
		LegacyBuckets: payloadSizeBuckets,
	}.ToPrometheus(), clientPayloadSizeLabels)
)

var (
	panicRecoverLabels = []string{
		methodLabel,
	}

	panicRecoverCounter = promauto.With(prometheusbpint.GlobalRegistry).NewCounterVec(prometheus.CounterOpts{
		Name: "thriftbp_server_recovered_panics_total",
		Help: "The number of panics recovered from thrift server handlers",
	}, panicRecoverLabels)
)

var (
	clientPoolLabels = []string{
		"thrift_pool",
	}

	clientPoolExhaustedCounter = promauto.With(prometheusbpint.GlobalRegistry).NewCounterVec(prometheus.CounterOpts{
		Name: "thriftbp_client_pool_exhaustions_total",
		Help: "The number of pool exhaustions for a thrift client pool",
	}, clientPoolLabels)

	clientPoolClosedConnectionsCounter = promauto.With(prometheusbpint.GlobalRegistry).NewCounterVec(prometheus.CounterOpts{
		Name: "thriftbp_client_pool_closed_connections_total",
		Help: "The number of times we closed the client after used it from the pool",
	}, clientPoolLabels)

	clientPoolReleaseErrorCounter = promauto.With(prometheusbpint.GlobalRegistry).NewCounterVec(prometheus.CounterOpts{
		Name: "thriftbp_client_pool_release_errors_total",
		Help: "The number of times we failed to release a client back to the pool",
	}, clientPoolLabels)

	clientPoolOpenerCounter = promauto.With(prometheusbpint.GlobalRegistry).NewCounterVec(prometheus.CounterOpts{
		Name: "thriftbp_client_pool_opener_calls_total",
		Help: "The number of calls to open a new connection for a thriftbp client pool",
	}, []string{
		"thrift_pool",
	})

	clientPoolGetsCounter = promauto.With(prometheusbpint.GlobalRegistry).NewCounterVec(prometheus.CounterOpts{
		Name: "thrift_client_pool_connection_gets_total",
		Help: "The number of times we tried to lease(get) from a thrift client pool",
	}, []string{
		"thrift_pool",
		"thrift_success",
	})

	clientPoolMaxSizeGauge = promauto.With(prometheusbpint.GlobalRegistry).NewGaugeVec(prometheus.GaugeOpts{
		Name: "thrift_client_pool_max_size",
		Help: "The configured max size of a thrift client pool",
	}, []string{"thrift_pool"})

	clientPoolPeakActiveConnectionsDesc = prometheus.NewDesc(
		"thrift_client_pool_peak_active_connections",
		"The lifetime max number of active (in-use) connections of a thrift client pool",
		[]string{"thrift_pool"},
		nil, // const labels
	)

	clientPoolActiveConnectionsDesc = prometheus.NewDesc(
		"thrift_client_pool_active_connections",
		"The number of active (in-use) connections of a thrift client pool",
		[]string{"thrift_pool"},
		nil, // const labels
	)

	clientPoolIdleClientsDesc = prometheus.NewDesc(
		"thrift_client_pool_idle_connections",
		"The number of idle (in-pool) clients of a thrift client pool",
		[]string{"thrift_pool"},
		nil, // const labels
	)
)

const (
	clientLabel = "thrift_client"
)

var (
	deadlineBudgetLabels = []string{
		methodLabel,
		clientLabel,
	}

	deadlineBudgetHisto = promauto.With(prometheusbpint.GlobalRegistry).NewHistogramVec(prometheusbp.HistogramOpts{
		Name: "thriftbp_server_extracted_deadline_budget_seconds",
		Help: "Baseplate deadline budget extracted from client set header",
	}.ToPrometheus(), deadlineBudgetLabels)
)

type clientPoolGaugeExporter struct {
	slug string
	pool clientpool.Pool

	activeConnections prometheusbpint.HighWatermarkValue
}

func (e *clientPoolGaugeExporter) Describe(ch chan<- *prometheus.Desc) {
	// All metrics are described dynamically.
}

func (e *clientPoolGaugeExporter) Collect(ch chan<- prometheus.Metric) {
	e.activeConnections.Set(int64(e.pool.NumActiveClients()))
	active, max := e.activeConnections.GetBoth()
	idle := float64(e.pool.NumAllocated())

	// MustNewConstMetric would only panic if there's a label mismatch, which we
	// have a unit test to cover.
	ch <- prometheus.MustNewConstMetric(
		clientPoolActiveConnectionsDesc,
		prometheus.GaugeValue,
		float64(active),
		e.slug,
	)
	ch <- prometheus.MustNewConstMetric(
		clientPoolPeakActiveConnectionsDesc,
		prometheus.GaugeValue,
		float64(max),
		e.slug,
	)
	ch <- prometheus.MustNewConstMetric(
		clientPoolIdleClientsDesc,
		prometheus.GaugeValue,
		idle,
		e.slug,
	)
}

func stringifyErrorType(err error) string {
	if err == nil {
		return ""
	}
	var te thrift.TException
	if errors.As(err, &te) && te.TExceptionType() == thrift.TExceptionTypeUnknown {
		// This usually means the error was wrapped by thrift.WrapTException,
		// try unwrap it.
		if unwrapped := errors.Unwrap(te); unwrapped != nil {
			err = unwrapped
		}
	}
	if err == context.Canceled {
		// Special handling of context.Canceled.
		// As of Go 1.19, context.Canceled is generated from errors.New,
		// so the type would be indistinguishable from other errors from errors.New.
		// Note that we intentionally used == instead of errors.Is here,
		// so that if it's wrapped context.Canceled we would still return the
		// wrapping type instead, which is usually more important info.
		return "context.Canceled"
	}
	return strings.TrimPrefix(fmt.Sprintf("%T", err), "*")
}
