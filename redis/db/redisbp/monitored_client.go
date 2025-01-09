package redisbp

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/reddit/baseplate.go/internal/prometheusbpint"
	"github.com/reddit/baseplate.go/metricsbp"
	"github.com/reddit/baseplate.go/redis/internal/redisprom"
)

// ErrReplicationFactorFailed returns when the cluster client wait function returns replica reached count
// that is less than desired replication factor
var ErrReplicationFactorFailed = errors.New("redisbp: failed to meet the requested replication factor")

// PoolStatser is an interface with PoolStats that reports pool related metrics
type PoolStatser interface {
	// PoolStats returns the stats of the underlying connection pool.
	PoolStats() *redis.PoolStats
}

func getDeploymentType(addr string) string {
	if strings.Contains(addr, "cache.amazonaws") {
		return "elasticache"
	} else {
		return "reddit"
	}
}

func getTargetCluster(addr string) string {
	if strings.Contains(addr, "cache.amazonaws") {
		return ""
	}

	// redis-<cluster name>.<vpc>.<region>.<postfix>.net:6379
	tokens := strings.Split(addr, ".")
	if len(tokens) != 5 {
		return ""
	}

	if name, found := strings.CutPrefix(tokens[0], "redis-"); found {
		return name
	}

	return ""
}

// NewMonitoredClient creates a new *redis.Client object with a redisbp.SpanHook
// attached that connects to a single Redis instance.
func NewMonitoredClient(name string, opt *redis.Options) *redis.Client {
	hook := SpanHook{
		ClientName: name,
		Type:       "standalone",
		Deployment: getDeploymentType(opt.Addr),
		Cluster:    getTargetCluster(opt.Addr),
		Database:   strconv.Itoa(opt.DB),
		promActive: &prometheusbpint.HighWatermarkGauge{
			HighWatermarkValue:   &prometheusbpint.HighWatermarkValue{},
			CurrGauge:            redisprom.ActiveConnectionsDesc,
			CurrGaugeLabelValues: []string{name},
			MaxGauge:             redisprom.PeakActiveConnectionsDesc,
			MaxGaugeLabelValues:  []string{name},
		},
	}
	if err := prometheusbpint.GlobalRegistry.Register(hook.promActive); err != nil {
		// Register should never fail because HighWatermarkGauge.Describe is a no-op,
		// but just in case.
		return nil
	}

	client := redis.NewClient(opt)
	client.AddHook(hook)
	redisprom.MaxSizeGauge.WithLabelValues(name).Set(float64(opt.PoolSize))

	if err := prometheusbpint.GlobalRegistry.Register(exporter{
		client: client,
		name:   name,
	}); err != nil {
		// Register should never fail because exporter.Describe is a no-op,
		// but just in case.
		client.Close()
		return nil
	}

	return client
}

// NewMonitoredFailoverClient creates a new failover *redis.Client using Redis
// Sentinel with a redisbp.SpanHook attached.
func NewMonitoredFailoverClient(name string, opt *redis.FailoverOptions) *redis.Client {
	hook := SpanHook{
		ClientName: name,
		Type:       "sentinel",
		Deployment: getDeploymentType(opt.SentinelAddrs[0]),
		Cluster:    getTargetCluster(opt.SentinelAddrs[0]),
		Database:   strconv.Itoa(opt.DB),
		promActive: &prometheusbpint.HighWatermarkGauge{
			HighWatermarkValue:   &prometheusbpint.HighWatermarkValue{},
			CurrGauge:            redisprom.ActiveConnectionsDesc,
			CurrGaugeLabelValues: []string{name},
			MaxGauge:             redisprom.PeakActiveConnectionsDesc,
			MaxGaugeLabelValues:  []string{name},
		},
	}
	if err := prometheusbpint.GlobalRegistry.Register(hook.promActive); err != nil {
		// Register should never fail because HighWatermarkGauge.Describe is a no-op,
		// but just in case.
		return nil
	}

	client := redis.NewFailoverClient(opt)
	client.AddHook(hook)
	redisprom.MaxSizeGauge.WithLabelValues(name).Set(float64(opt.PoolSize))

	if err := prometheusbpint.GlobalRegistry.Register(exporter{
		client: client,
		name:   name,
	}); err != nil {
		// Register should never fail because exporter.Describe is a no-op,
		// but just in case.
		client.Close()
		return nil
	}

	return client
}

// ClusterClient extends redis cluster client with a functional Wait function
type ClusterClient struct {
	*redis.ClusterClient
}

// WaitArgs enclose inputs for Wait command into a struct
type WaitArgs struct {
	Key         string
	NumReplicas int
	Timeout     time.Duration
}

// Wait blocks the current client until all the previous write commands are
// successfully transferred and acknowledged by at least the specified number of replicas.
func (cc *ClusterClient) Wait(ctx context.Context, args WaitArgs) (replicas int64, err error) {
	if args.NumReplicas <= 0 {
		return 0, nil
	}

	client, err := cc.ClusterClient.MasterForKey(ctx, args.Key)
	if err != nil {
		return 0, fmt.Errorf("redisbp: error while trying to retrieve master from key: %w", err)
	}

	replicas, err = client.Wait(ctx, args.NumReplicas, args.Timeout).Result()
	if err != nil {
		return 0, fmt.Errorf("redisbp: error while trying to apply replication factor: %w", err)
	}

	if int(replicas) < args.NumReplicas {
		return replicas, fmt.Errorf("%w: %d/%d", ErrReplicationFactorFailed, replicas, args.NumReplicas)
	}

	return
}

// NewMonitoredClusterClient creates a new *redis.ClusterClient object with a
// redisbp.SpanHook attached.
func NewMonitoredClusterClient(name string, opt *redis.ClusterOptions) *ClusterClient {
	hook := SpanHook{
		ClientName: name,
		Type:       "cluster",
		Deployment: getDeploymentType(opt.Addrs[0]),
		Cluster:    getTargetCluster(opt.Addrs[0]),
		Database:   "", // We don't have that for cluster clients
		promActive: &prometheusbpint.HighWatermarkGauge{
			HighWatermarkValue:   &prometheusbpint.HighWatermarkValue{},
			CurrGauge:            redisprom.ActiveConnectionsDesc,
			CurrGaugeLabelValues: []string{name},
			MaxGauge:             redisprom.PeakActiveConnectionsDesc,
			MaxGaugeLabelValues:  []string{name},
		},
	}
	if err := prometheusbpint.GlobalRegistry.Register(hook.promActive); err != nil {
		// Register should never fail because HighWatermarkGauge.Describe is a no-op,
		// but just in case.
		return nil
	}

	client := redis.NewClusterClient(opt)
	client.AddHook(hook)
	redisprom.MaxSizeGauge.WithLabelValues(name).Set(float64(opt.PoolSize))

	if err := prometheusbpint.GlobalRegistry.Register(exporter{
		client: client,
		name:   name,
	}); err != nil {
		// Register should never fail because exporter.Describe is a no-op,
		// but just in case.
		client.Close()
		return nil
	}

	return &ClusterClient{client}
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
//
// Deprecated: Statsd metrics are deprecated. Prometheus pool stats metrics are
// always reported with a monitored client.
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
