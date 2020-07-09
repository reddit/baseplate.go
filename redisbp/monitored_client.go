package redisbp

import (
	"context"
	"io"
	"time"

	"github.com/go-redis/redis/v7"
	"github.com/reddit/baseplate.go/metricsbp"
)

// MonitoredCmdable wraps the redis.Cmdable interface and adds additional methods
// to support integrating with Baseplate.go SpanHooks.
type MonitoredCmdable interface {
	// Close should generally not be called directly on a MonitoredCmdable, since
	// they are meant to be shared and long lived.  It will be called by
	// MonitoredCmdableFactory.Close which should be called when a server is shut
	// down.
	io.Closer
	redis.Cmdable

	// PoolStats returns the stats of the underlying connection pool.
	PoolStats() *redis.PoolStats

	// AddHook adds a hook onto the object.
	//
	// Note most redis.Cmdable objects already implement this but it is not a
	// part of the redis.Cmdable interface.
	AddHook(hook redis.Hook)

	// WithMonitoredContext returns a clone of the MonitoredCmdable with its
	// context set to the provided one.
	//
	// This is similar to the WithContext method that many redis.Cmdable objects
	// implement, but this returns a MonitoredCmdable rather than a pointer to
	// the exact type.  Also note that WithContext is not a part of the
	// redis.Cmdable interface.
	WithMonitoredContext(ctx context.Context) MonitoredCmdable
}

type monitoredClient struct {
	*redis.Client
}

func (c *monitoredClient) WithMonitoredContext(ctx context.Context) MonitoredCmdable {
	return &monitoredClient{Client: c.Client.WithContext(ctx)}
}

type monitoredCluster struct {
	*redis.ClusterClient
}

func (c *monitoredCluster) WithMonitoredContext(ctx context.Context) MonitoredCmdable {
	return &monitoredCluster{ClusterClient: c.ClusterClient.WithContext(ctx)}
}

// NewMonitoredClientFactory creates a MonitoredCmdableFactory for a redis.Client
// object.
//
// This may connect to a single redis instance, or be a failover client using
// Redis Sentinel.
func NewMonitoredClientFactory(name string, client *redis.Client) MonitoredCmdableFactory {
	return newMonitoredCmdableFactory(name, &monitoredClient{Client: client})
}

// NewMonitoredClusterFactory creates a MonitoredCmdableFactory for a
// redis.ClusterClient object.
func NewMonitoredClusterFactory(name string, client *redis.ClusterClient) MonitoredCmdableFactory {
	return newMonitoredCmdableFactory(name, &monitoredCluster{ClusterClient: client})
}

// MonitoredCmdableFactory is used to create Redis clients that are monitored by
// a SpanHook.
//
// This is intended for use by a top-level Service interface where you use it on
// each new request to build a monitored client to inject into the actual
// request handler.
//
// A MonitoredCmdableFactory should be created using one of the New methods
// provided in this package.
type MonitoredCmdableFactory struct {
	client MonitoredCmdable
	name   string
}

func newMonitoredCmdableFactory(name string, client MonitoredCmdable) MonitoredCmdableFactory {
	client.AddHook(SpanHook{ClientName: name})
	return MonitoredCmdableFactory{client: client, name: name}
}

// BuildClient returns a new MonitoredCmdable with its context set to the
// provided one.
func (f MonitoredCmdableFactory) BuildClient(ctx context.Context) MonitoredCmdable {
	return f.client.WithMonitoredContext(ctx)
}

// Close closes the underlying MonitoredCmdable, which will close the underlying
// connection pool, closing out any clients created with the factory.
func (f MonitoredCmdableFactory) Close() error {
	return f.client.Close()
}

// MonitorPoolStats publishes stats for the underlying Redis client pool at the
// rate defined by metricsbp.SysStatsTickerInterval using the given metrics
// client.
//
// It is recommended that you call this in a separate goroutine as it will run
// until it is stopped.  It will stop when the given metrics client's context is
// Done().
func (f MonitoredCmdableFactory) MonitorPoolStats(metrics metricsbp.Statsd, tags metricsbp.Tags) {
	t := tags.AsStatsdTags()
	prefix := f.name + ".pool"
	hitsGauge := metrics.RuntimeGauge(prefix + ".hits").With(t...)
	missesGauge := metrics.RuntimeGauge(prefix + ".misses").With(t...)
	timeoutsGauge := metrics.RuntimeGauge(prefix + ".timeouts").With(t...)
	totalConnectionsGauge := metrics.RuntimeGauge(prefix + ".connections.total").With(t...)
	idleConnectionsGauge := metrics.RuntimeGauge(prefix + ".connections.idle").With(t...)
	staleConnectionsGauge := metrics.RuntimeGauge(prefix + ".connections.stale").With(t...)
	client := f.BuildClient(context.TODO())
	ticker := time.NewTicker(metricsbp.SysStatsTickerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-metrics.Ctx().Done():
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

var (
	_ MonitoredCmdable = (*monitoredClient)(nil)
	_ MonitoredCmdable = (*monitoredCluster)(nil)
	_ io.Closer        = MonitoredCmdableFactory{}
	_ io.Closer        = (*MonitoredCmdableFactory)(nil)
)
