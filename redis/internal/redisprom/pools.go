package redisprom

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	//lint:ignore SA1019 This library is internal only, not actually deprecated
	"github.com/reddit/baseplate.go/internalv2compat"
)

const redisPoolLabel = "redis_pool"

var (
	MaxSizeGauge = promauto.With(internalv2compat.GlobalRegistry).NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "redis_client_max_size",
			Help: "configured maximum number of clients to keep in the pool (for showing % used in dashboards)",
		},
		[]string{redisPoolLabel},
	)
	ActiveConnectionsDesc = prometheus.NewDesc(
		"redis_client_active_connections",
		"current number of connections 'leased' from the pool. MAY be greater than size in the event that the implementation allows for using more connections than in the pool",
		[]string{redisPoolLabel},
		prometheus.Labels{},
	)
	IdleConnectionsDesc = prometheus.NewDesc(
		"redis_client_idle_connections",
		"current number of connections 'idle' (not leased from the pool) and ready to be used",
		[]string{redisPoolLabel},
		prometheus.Labels{},
	)
	PeakActiveConnectionsDesc = prometheus.NewDesc(
		"redis_client_peak_active_connections",
		"maximum number of connections simultaneously 'leased' from the pool since process start",
		[]string{redisPoolLabel},
		prometheus.Labels{},
	)
	TotalConnectionGetsDesc = prometheus.NewDesc(
		"redis_client_total_connection_gets",
		"incremented each time a connection is 'leased' from the pool",
		[]string{redisPoolLabel},
		prometheus.Labels{},
	)
)
