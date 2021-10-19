package thriftbp

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	thriftLabels = []string{
		"thrift_service",
		"thrift_method",
		"thrift_success",
		"thrift_exception_type",
		"thrift_baseplate_status",
		"thrift_baseplate_status_code",
	}

	latencyDistribution = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "thrift_server_handling_seconds",
		Help:    "RPC latencies",
		Buckets: prometheus.ExponentialBuckets(0.0001, 1.5, 26), // 100us ~ 2.5s
	}, thriftLabels)

	rpcStatusCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "thrift_server_handled_total",
		Help: "Total RPC request count",
	}, thriftLabels)

	activeRequests = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "thrift_active_requests",
		Help: "The number of requests being handled by the service.",
	}, thriftLabels)
)

func labelsZeroValue(labels []string) prometheus.Labels {
	lbs := prometheus.Labels{}
	for _, lb := range labels {
		lbs[lb] = ""
	}
	return lbs
}

type PrometheusMetric struct {
	service string
	labels  prometheus.Labels
}

func NewPromethesuMetric() *PrometheusMetric {
	return &PrometheusMetric{
		labels: labelsZeroValue(thriftLabels),
	}
}

var ThriftPrometheusMetrics = NewPromethesuMetric()

func (m *PrometheusMetric) SetPromethesuService(serviceName string) {
	m.service = serviceName
}

func (m *PrometheusMetric) setLabels(method, success, exception, bpStatus, bpStatusCode string) {
	m.labels["thrift_service"] = m.service
	m.labels["thrift_method"] = method
	m.labels["thrift_success"] = success
	m.labels["thrift_exception_type"] = exception
	m.labels["thrift_baseplate_status"] = bpStatus
	m.labels["thrift_baseplate_status_code"] = bpStatusCode
}

func (m *PrometheusMetric) observeLatencyDistribution(start time.Time) {
	latencyDistribution.With(m.labels).Observe(time.Since(start).Seconds())
}

func (m *PrometheusMetric) incRPCStatusCounter() {
	rpcStatusCounter.With(m.labels).Inc()
}

func (m *PrometheusMetric) incActiveRequests() {
	activeRequests.With(m.labels).Inc()
}

func (m *PrometheusMetric) decActiveRequests() {
	activeRequests.With(m.labels).Dec()
}
