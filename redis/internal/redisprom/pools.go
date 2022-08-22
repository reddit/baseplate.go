package redisprom

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const RedisPoolLabel = "redis_pool"

var (
	MaxSizeGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "redis_client_max_size",
			Help: "configured maximum number of clients to keep in the pool (for showing % used in dashboards)",
		},
		[]string{RedisPoolLabel},
	)
	ActiveConnectionsDesc = prometheus.NewDesc(
		"redis_client_active_connections",
		"current number of connections 'leased' from the pool. MAY be greater than size in the event that the implementation allows for using more connections than in the pool",
		[]string{RedisPoolLabel},
		prometheus.Labels{},
	)
	IdleConnectionsDesc = prometheus.NewDesc(
		"redis_client_idle_connections",
		"current number of connections 'idle' (not leased from the pool) and ready to be used",
		[]string{RedisPoolLabel},
		prometheus.Labels{},
	)
	PeakActiveConnectionsGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "redis_client_peak_active_connections",
			Help: "maximum number of connections simultaneously 'leased' from the pool since process start",
		},
		[]string{RedisPoolLabel},
	)
	TotalConnectionGetsDesc = prometheus.NewDesc(
		"redis_client_total_connection_gets",
		"incremented each time a connection is 'leased' from the pool",
		[]string{RedisPoolLabel},
		prometheus.Labels{},
	)
)
