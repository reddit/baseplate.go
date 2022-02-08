package redisbp

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	promNamespace = "redisbp"
	subsystemPool = "pool"

	nameLabel = "pool"
)

// exporter provides an interface for Prometheus metrics.
type exporter struct {
	client PoolStatser
	name   string

	poolHitsCounterDesc     *prometheus.Desc
	poolMissesCounterDesc   *prometheus.Desc
	poolTimeoutsCounterDesc *prometheus.Desc
	totalConnectionsDesc    *prometheus.Desc
	idleConnectionsDesc     *prometheus.Desc
	staleConnectionsDesc    *prometheus.Desc
}

func newExporter(client PoolStatser, name string) *exporter {
	labels := []string{
		nameLabel,
	}

	return &exporter{
		client: client,
		name:   name,

		// Upstream docs: https://pkg.go.dev/github.com/go-redis/redis/v8/internal/pool#Stats

		// Counters.
		poolHitsCounterDesc: prometheus.NewDesc(
			prometheus.BuildFQName(promNamespace, subsystemPool, "hits_total"),
			"Number of times free connection was found in the pool",
			labels,
			nil,
		),
		poolMissesCounterDesc: prometheus.NewDesc(
			prometheus.BuildFQName(promNamespace, subsystemPool, "misses_total"),
			"Number of times free connection was NOT found in the pool",
			labels,
			nil,
		),
		poolTimeoutsCounterDesc: prometheus.NewDesc(
			prometheus.BuildFQName(promNamespace, subsystemPool, "timeouts_total"),
			"Number of times a wait timeout occurred",
			labels,
			nil,
		),

		// Gauges.
		totalConnectionsDesc: prometheus.NewDesc(
			prometheus.BuildFQName(promNamespace, subsystemPool, "connections"),
			"Number of connections in this redisbp pool",
			labels,
			nil,
		),
		idleConnectionsDesc: prometheus.NewDesc(
			prometheus.BuildFQName(promNamespace, subsystemPool, "idle_connections"),
			"Number of idle connections in this redisbp pool",
			labels,
			nil,
		),
		staleConnectionsDesc: prometheus.NewDesc(
			prometheus.BuildFQName(promNamespace, subsystemPool, "stale_connections"),
			"Number of stale connections in this redisbp pool",
			labels,
			nil,
		),
	}
}

// Describe implements the prometheus.Collector interface.
func (e *exporter) Describe(ch chan<- *prometheus.Desc) {
	// All metrics are described dynamically.
}

// Collect implements prometheus.Collector.
func (e *exporter) Collect(ch chan<- prometheus.Metric) {
	stats := e.client.PoolStats()

	// Counters.
	ch <- prometheus.MustNewConstMetric(
		e.poolHitsCounterDesc,
		prometheus.CounterValue,
		float64(stats.Hits),
		e.name,
	)
	ch <- prometheus.MustNewConstMetric(
		e.poolMissesCounterDesc,
		prometheus.CounterValue,
		float64(stats.Misses),
		e.name,
	)
	ch <- prometheus.MustNewConstMetric(
		e.poolTimeoutsCounterDesc,
		prometheus.CounterValue,
		float64(stats.Timeouts),
		e.name,
	)

	// Gauges.
	ch <- prometheus.MustNewConstMetric(
		e.totalConnectionsDesc,
		prometheus.GaugeValue,
		float64(stats.TotalConns),
		e.name,
	)
	ch <- prometheus.MustNewConstMetric(
		e.idleConnectionsDesc,
		prometheus.GaugeValue,
		float64(stats.IdleConns),
		e.name,
	)
	ch <- prometheus.MustNewConstMetric(
		e.staleConnectionsDesc,
		prometheus.GaugeValue,
		float64(stats.StaleConns),
		e.name,
	)
}
