package thriftbp

import (
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

	serverLatencyDistribution = promauto.With(prometheusbpint.GlobalRegistry).NewHistogramVec(prometheus.HistogramOpts{
		Name:    "thrift_server_latency_seconds",
		Help:    "RPC latencies",
		Buckets: prometheusbp.DefaultLatencyBuckets,
	}, serverLatencyLabels)

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

	clientLatencyDistribution = promauto.With(prometheusbpint.GlobalRegistry).NewHistogramVec(prometheus.HistogramOpts{
		Name:    "thrift_client_latency_seconds",
		Help:    "RPC latencies",
		Buckets: prometheusbp.DefaultLatencyBuckets,
	}, clientLatencyLabels)

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

const (
	// Note that this is not used by the prometheus metrics defined in Baseplate
	// spec.
	promNamespace = "thriftbp"

	subsystemServer     = "server"
	subsystemTTLClient  = "ttl_client"
	subsystemClientPool = "client_pool"
)

var (
	serverConnectionsGauge = promauto.With(prometheusbpint.GlobalRegistry).NewGauge(prometheus.GaugeOpts{
		Namespace: promNamespace,
		Subsystem: subsystemServer,
		Name:      "connections",
		Help:      "The number of client connections established to the service",
	})
)

var (
	ttlClientReplaceLabels = []string{
		clientNameLabel,
		successLabel,
	}

	ttlClientReplaceCounter = promauto.With(prometheusbpint.GlobalRegistry).NewCounterVec(prometheus.CounterOpts{
		Namespace: promNamespace,
		Subsystem: subsystemTTLClient,
		Name:      "connection_housekeeping_total",
		Help:      "Total connection housekeeping (replacing the connection in the background) done in thrift ttlClient",
	}, ttlClientReplaceLabels)
)

const (
	protoLabel = "thrift_proto"
)

var (
	payloadSizeLabels = []string{
		methodLabel,
		protoLabel,
	}

	// 8 bytes to 4 mebibytes
	// some endpoints can have very small, almost zero payloads (for example,
	// is_healthy), but we do have some endpoints with very large payloads
	// (up to ~500 KiB).
	payloadSizeBuckets = prometheus.ExponentialBuckets(8, 2, 20)

	payloadSizeRequestBytes = promauto.With(prometheusbpint.GlobalRegistry).NewHistogramVec(prometheus.HistogramOpts{
		Namespace: promNamespace,
		Subsystem: subsystemServer,
		Name:      "request_payload_size_bytes",
		Help:      "The size of thrift request payloads",
		Buckets:   payloadSizeBuckets,
	}, payloadSizeLabels)

	payloadSizeResponseBytes = promauto.With(prometheusbpint.GlobalRegistry).NewHistogramVec(prometheus.HistogramOpts{
		Namespace: promNamespace,
		Subsystem: subsystemServer,
		Name:      "response_payload_size_bytes",
		Help:      "The size of thrift response payloads",
		Buckets:   payloadSizeBuckets,
	}, payloadSizeLabels)
)

var (
	panicRecoverLabels = []string{
		methodLabel,
	}

	panicRecoverCounter = promauto.With(prometheusbpint.GlobalRegistry).NewCounterVec(prometheus.CounterOpts{
		Namespace: promNamespace,
		Subsystem: subsystemServer,
		Name:      "panic_recover_total",
		Help:      "The number of panics recovered from thrift server handlers",
	}, panicRecoverLabels)
)

var (
	clientPoolLabels = []string{
		clientNameLabel, // TODO: Remove after the next release (v0.9.12)
		"thrift_pool",
	}

	clientPoolExhaustedCounter = promauto.With(prometheusbpint.GlobalRegistry).NewCounterVec(prometheus.CounterOpts{
		Namespace: promNamespace,
		Subsystem: subsystemClientPool,
		Name:      "exhausted_total",
		Help:      "The number of pool exhaustion for a thrift client pool",
	}, clientPoolLabels)

	clientPoolClosedConnectionsCounter = promauto.With(prometheusbpint.GlobalRegistry).NewCounterVec(prometheus.CounterOpts{
		Namespace: promNamespace,
		Subsystem: subsystemClientPool,
		Name:      "closed_connections_total",
		Help:      "The number of times we closed the client after used it from the pool",
	}, clientPoolLabels)

	clientPoolReleaseErrorCounter = promauto.With(prometheusbpint.GlobalRegistry).NewCounterVec(prometheus.CounterOpts{
		Namespace: promNamespace,
		Subsystem: subsystemClientPool,
		Name:      "release_error_total",
		Help:      "The number of times we failed to release a client back to the pool",
	}, clientPoolLabels)

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

	// TODO: Remove after the next release (v0.9.12)
	legacyClientPoolActiveConnectionsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(promNamespace, subsystemClientPool, "active_connections"),
		"The number of active (in-use) connections of a thrift client pool",
		clientPoolLabels,
		nil, // const labels
	)

	// TODO: Remove after the next release (v0.9.12)
	legacyClientPoolAllocatedClientsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(promNamespace, subsystemClientPool, "allocated_clients"),
		"The number of allocated (in-pool) clients of a thrift client pool",
		clientPoolLabels,
		nil, // const labels
	)

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

	deadlineBudgetHisto = promauto.With(prometheusbpint.GlobalRegistry).NewHistogramVec(prometheus.HistogramOpts{
		Namespace: promNamespace,
		Subsystem: subsystemServer,
		Name:      "extracted_deadline_budget_seconds",
		Help:      "Baseplate deadline budget extracted from client set header",
		Buckets:   prometheusbp.DefaultLatencyBuckets,
	}, deadlineBudgetLabels)
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
		legacyClientPoolActiveConnectionsDesc,
		prometheus.GaugeValue,
		float64(active),
		e.slug,
		e.slug,
	)
	ch <- prometheus.MustNewConstMetric(
		legacyClientPoolAllocatedClientsDesc,
		prometheus.GaugeValue,
		idle,
		e.slug,
		e.slug,
	)

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
	return strings.TrimPrefix(fmt.Sprintf("%T", err), "*")
}
