package thriftbp

import (
	"errors"
	"fmt"
	"strings"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/reddit/baseplate.go/clientpool"
	"github.com/reddit/baseplate.go/prometheusbp"
)

const (
	methodLabel              = "thrift_method"
	successLabel             = "thrift_success"
	exceptionLabel           = "thrift_exception_type"
	baseplateStatusLabel     = "thrift_baseplate_status"
	baseplateStatusCodeLabel = "thrift_baseplate_status_code"
	serverSlugLabel          = "thrift_slug"
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

	serverTotalRequestLabels = []string{
		methodLabel,
		successLabel,
		exceptionLabel,
		baseplateStatusLabel,
		baseplateStatusCodeLabel,
	}

	serverTotalRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "thrift_server_requests_total",
		Help: "Total RPC request count",
	}, serverTotalRequestLabels)

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
		serverSlugLabel,
	}

	clientLatencyDistribution = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "thrift_client_latency_seconds",
		Help:    "RPC latencies",
		Buckets: prometheusbp.DefaultBuckets,
	}, clientLatencyLabels)

	clientTotalRequestLabels = []string{
		methodLabel,
		successLabel,
		exceptionLabel,
		baseplateStatusLabel,
		baseplateStatusCodeLabel,
		serverSlugLabel,
	}

	clientTotalRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "thrift_client_requests_total",
		Help: "Total RPC request count",
	}, clientTotalRequestLabels)

	clientActiveRequestsLabels = []string{
		methodLabel,
		serverSlugLabel,
	}

	clientActiveRequests = promauto.NewGaugeVec(prometheus.GaugeOpts{
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
	serverConnectionsGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: promNamespace,
		Subsystem: subsystemServer,
		Name:      "connections",
		Help:      "The number of client connections established to the service",
	})
)

var (
	ttlClientReplaceLabels = []string{
		serverSlugLabel,
		successLabel,
	}

	ttlClientReplaceCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: promNamespace,
		Subsystem: subsystemTTLClient,
		Name:      "connection_houskeeping_total",
		Help:      "Total connection housekeeping (replace the connection in the background) done in thrift ttlClient",
	}, ttlClientReplaceLabels)
)

const (
	protoLabel = "proto"
)

var (
	payloadSizeLabels = []string{
		methodLabel,
		protoLabel,
	}

	// 8 bytes to 8 megabytes
	// some endpoints can have very small, almost zero payloads (for example,
	// is_healthy), but we do have some endpoints with very large payloads
	// (up to ~400KB).
	payloadSizeBuckets = prometheus.ExponentialBuckets(8, 2, 20)

	payloadSizeRequestBytes = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: promNamespace,
		Subsystem: subsystemServer,
		Name:      "request_payload_size_bytes",
		Help:      "The size of thrift request payloads",
		Buckets:   payloadSizeBuckets,
	}, payloadSizeLabels)

	payloadSizeResponseBytes = promauto.NewHistogramVec(prometheus.HistogramOpts{
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

	panicRecoverCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: promNamespace,
		Subsystem: subsystemServer,
		Name:      "panic_recover_total",
		Help:      "The number of panics recovered from thrift server handlers",
	}, panicRecoverLabels)
)

var (
	clientPoolLabels = []string{
		serverSlugLabel,
	}

	clientPoolExhaustedCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: promNamespace,
		Subsystem: subsystemClientPool,
		Name:      "exhausted_total",
		Help:      "The number of pool exhaustion for a thrift client pool",
	}, clientPoolLabels)

	clientPoolClosedConnectionsCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: promNamespace,
		Subsystem: subsystemClientPool,
		Name:      "closed_connections_total",
		Help:      "The number of times we closed the client after used it from the pool",
	}, clientPoolLabels)

	clientPoolReleaseErrorCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: promNamespace,
		Subsystem: subsystemClientPool,
		Name:      "release_error_total",
		Help:      "The number of times we failed to release a client back to the pool",
	}, clientPoolLabels)

	clientPoolActiveConnectionsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(promNamespace, subsystemClientPool, "active_connections"),
		"The number of active (in-use) connections of a thrift client pool",
		clientPoolLabels,
		nil, // const labels
	)

	clientPoolAllocatedClientsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(promNamespace, subsystemClientPool, "allocated_clients"),
		"The number of allocated (in-pool) clients of a thrift client pool",
		clientPoolLabels,
		nil, // const labels
	)
)

type clientPoolGaugeExporter struct {
	slug string
	pool clientpool.Pool
}

func (e clientPoolGaugeExporter) Describe(ch chan<- *prometheus.Desc) {
	// All metrics are described dynamically.
}

func (e clientPoolGaugeExporter) Collect(ch chan<- prometheus.Metric) {
	// MustNewConstMetric would only panic if there's a label mismatch, which we
	// have a unit test to cover.
	ch <- prometheus.MustNewConstMetric(
		clientPoolActiveConnectionsDesc,
		prometheus.GaugeValue,
		float64(e.pool.NumActiveClients()),
		e.slug,
	)
	ch <- prometheus.MustNewConstMetric(
		clientPoolAllocatedClientsDesc,
		prometheus.GaugeValue,
		float64(e.pool.NumAllocated()),
		e.slug,
	)
}

func stringifyErrorType(err error) string {
	if err == nil {
		return ""
	}
	return strings.TrimPrefix(fmt.Sprintf("%T", unwrapTException(err)), "*")
}

func unwrapTException(err error) error {
	var te thrift.TException
	if errors.As(err, &te) && te.TExceptionType() == thrift.TExceptionTypeUnknown {
		// This usually means the error was wrapped by thrift.WrapTException,
		// try unwrap it.
		if unwrapped := errors.Unwrap(te); unwrapped != nil {
			err = unwrapped
		}
	}
	return err
}
