package redisbp

import (
	"context"

	"github.com/go-redis/redis/v7"
)

// MonitoredCmdable wraps the redis.Cmdable interface and adds additional methods
// to support integrating with baseplate.go SpanHooks.
type MonitoredCmdable interface {
	redis.Cmdable

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

type monitoredRing struct {
	*redis.Ring
}

func (c *monitoredRing) WithMonitoredContext(ctx context.Context) MonitoredCmdable {
	return &monitoredRing{Ring: c.Ring.WithContext(ctx)}
}

// MonitoredCmdableFactory is used to create Redis clients that are monitored by
// a SpanHook.
//
// A MonitoredCmdableFactory should be created using one of the New methods
// provided in this package.
type MonitoredCmdableFactory struct {
	client MonitoredCmdable
}

func newMonitoredCmdableFactory(name string, client MonitoredCmdable) MonitoredCmdableFactory {
	client.AddHook(SpanHook{ClientName: name})
	return MonitoredCmdableFactory{client: client}
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

// NewMonitoredRingFactory creates a MonitoredCmdableFactory for a redis.Ring
// object.
func NewMonitoredRingFactory(name string, client *redis.Ring) MonitoredCmdableFactory {
	return newMonitoredCmdableFactory(name, &monitoredRing{Ring: client})
}

// BuildClient returns a new MonitoredCmdable with its context set to the
// provided one.
func (f MonitoredCmdableFactory) BuildClient(ctx context.Context) MonitoredCmdable {
	return f.client.WithMonitoredContext(ctx)
}

var (
	_ MonitoredCmdable = (*monitoredClient)(nil)
	_ MonitoredCmdable = (*monitoredCluster)(nil)
	_ MonitoredCmdable = (*monitoredRing)(nil)
)
