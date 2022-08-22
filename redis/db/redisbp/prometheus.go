package redisbp

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	//lint:ignore SA1019 This library is internal only, not actually deprecated
	"github.com/reddit/baseplate.go/internalv2compat"
	"github.com/reddit/baseplate.go/prometheusbp"
	"github.com/reddit/baseplate.go/redis/internal/redisprom"
)

const (
	promNamespace = "redisbp"
	subsystemPool = "pool"

	nameLabel    = "redis_pool"
	commandLabel = "redis_command"
	successLabel = "redis_success"
)

var (
	promLabels = []string{
		nameLabel,
	}

	// Counters.
	poolHitsCounterDesc = prometheus.NewDesc(
		prometheus.BuildFQName(promNamespace, subsystemPool, "hits_total"),
		"Number of times free connection was found in the pool",
		promLabels,
		nil,
	)
	poolMissesCounterDesc = prometheus.NewDesc(
		prometheus.BuildFQName(promNamespace, subsystemPool, "misses_total"),
		"Number of times free connection was NOT found in the pool",
		promLabels,
		nil,
	)
	poolTimeoutsCounterDesc = prometheus.NewDesc(
		prometheus.BuildFQName(promNamespace, subsystemPool, "timeouts_total"),
		"Number of times a wait timeout occurred",
		promLabels,
		nil,
	)
	staleConnectionsCounterDesc = prometheus.NewDesc(
		prometheus.BuildFQName(promNamespace, subsystemPool, "stale_connections_total"),
		"Number of stale connections removed from this redisbp pool",
		promLabels,
		nil,
	)

	// Gauges.
	totalConnectionsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(promNamespace, subsystemPool, "connections"),
		"Number of connections in this redisbp pool",
		promLabels,
		nil,
	)
	idleConnectionsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(promNamespace, subsystemPool, "idle_connections"),
		"Number of idle connections in this redisbp pool",
		promLabels,
		nil,
	)
)

var (
	latencyLabels = []string{
		nameLabel,
		commandLabel,
		successLabel,
	}

	latencyTimer = promauto.With(internalv2compat.GlobalRegistry).NewHistogramVec(prometheus.HistogramOpts{
		Namespace: promNamespace,
		Name:      "latency_seconds",
		Help:      "Latency of redis operations",
		Buckets:   prometheusbp.DefaultBuckets,
	}, latencyLabels)
)

// exporter provides an interface for Prometheus metrics.
type exporter struct {
	client PoolStatser
	name   string
}

// Describe implements the prometheus.Collector interface.
func (e exporter) Describe(ch chan<- *prometheus.Desc) {
	// All metrics are described dynamically.
}

// Collect implements prometheus.Collector.
func (e exporter) Collect(ch chan<- prometheus.Metric) {
	stats := e.client.PoolStats()

	// Counters.
	ch <- prometheus.MustNewConstMetric(
		poolHitsCounterDesc,
		prometheus.CounterValue,
		float64(stats.Hits),
		e.name,
	)
	ch <- prometheus.MustNewConstMetric(
		poolMissesCounterDesc,
		prometheus.CounterValue,
		float64(stats.Misses),
		e.name,
	)
	ch <- prometheus.MustNewConstMetric(
		poolTimeoutsCounterDesc,
		prometheus.CounterValue,
		float64(stats.Timeouts),
		e.name,
	)
	ch <- prometheus.MustNewConstMetric(
		staleConnectionsCounterDesc,
		prometheus.CounterValue,
		float64(stats.StaleConns),
		e.name,
	)

	// Gauges.
	ch <- prometheus.MustNewConstMetric(
		totalConnectionsDesc,
		prometheus.GaugeValue,
		float64(stats.TotalConns),
		e.name,
	)
	ch <- prometheus.MustNewConstMetric(
		idleConnectionsDesc,
		prometheus.GaugeValue,
		float64(stats.IdleConns),
		e.name,
	)

	// Baseplate spec
	ch <- prometheus.MustNewConstMetric(
		redisprom.IdleConnectionsDesc,
		prometheus.GaugeValue,
		float64(stats.IdleConns),
		e.name,
	)
	ch <- prometheus.MustNewConstMetric(
		redisprom.TotalConnectionGetsDesc,
		prometheus.CounterValue,
		float64(stats.Hits),
		e.name,
	)
}
