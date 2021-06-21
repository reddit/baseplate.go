package redisbp

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/reddit/baseplate.go/metricsbp"
)

// PoolStatser is an interface with PoolStats that reports pool related metrics
type PoolStatser interface {
	// PoolStats returns the stats of the underlying connection pool.
	PoolStats() *redis.PoolStats
}

// NewMonitoredClient creates a new *redis.Client object with a redisbp.SpanHook
// attached that connects to a single Redis instance.
func NewMonitoredClient(name string, opt *redis.Options) *redis.Client {
	client := redis.NewClient(opt)
	client.AddHook(SpanHook{ClientName: name})
	return client
}

// NewMonitoredFailoverClient creates a new failover *redis.Client using Redis
// Sentinel with a redisbp.SpanHook attached.
func NewMonitoredFailoverClient(name string, opt *redis.FailoverOptions) *redis.Client {
	client := redis.NewFailoverClient(opt)
	client.AddHook(SpanHook{ClientName: name})
	return client
}

// NewMonitoredClusterClient creates a new *redis.ClusterClient object with a
// redisbp.SpanHook attached.
func NewMonitoredClusterClient(name string, opt *redis.ClusterOptions) *redis.ClusterClient {
	client := redis.NewClusterClient(opt)
	client.AddHook(SpanHook{ClientName: name})
	return client
}

// MonitorPoolStats publishes stats for the underlying Redis client pool at the
// rate defined by metricsbp.SysStatsTickerInterval using metricsbp.M.
//
// It is recommended that you call this in a separate goroutine as it will run
// until it is stopped.  It will stop when the given context is Done()
//
// Ex:
//
//	go factory.MonitorPoolStats(metricsbp.M.Ctx(), tags)
func MonitorPoolStats(ctx context.Context, client PoolStatser, name string, tags metricsbp.Tags) {
	t := tags.AsStatsdTags()
	prefix := name + ".pool"
	hitsGauge := metricsbp.M.RuntimeGauge(prefix + ".hits").With(t...)
	missesGauge := metricsbp.M.RuntimeGauge(prefix + ".misses").With(t...)
	timeoutsGauge := metricsbp.M.RuntimeGauge(prefix + ".timeouts").With(t...)
	totalConnectionsGauge := metricsbp.M.RuntimeGauge(prefix + ".connections.total").With(t...)
	idleConnectionsGauge := metricsbp.M.RuntimeGauge(prefix + ".connections.idle").With(t...)
	staleConnectionsGauge := metricsbp.M.RuntimeGauge(prefix + ".connections.stale").With(t...)
	ticker := time.NewTicker(metricsbp.SysStatsTickerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			stats := client.PoolStats()
			hitsGauge.Set(float64(stats.Hits))
			missesGauge.Set(float64(stats.Misses))
			timeoutsGauge.Set(float64(stats.Timeouts))
			totalConnectionsGauge.Set(float64(stats.TotalConns))
			idleConnectionsGauge.Set(float64(stats.IdleConns))
			staleConnectionsGauge.Set(float64(stats.StaleConns))
		}
	}
}
