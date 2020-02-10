package redisbp_test

import (
	"context"

	"github.com/go-redis/redis/v7"

	"github.com/reddit/baseplate.go/redisbp"
	"github.com/reddit/baseplate.go/tracing"
)

// This example demonstrates how to use a MonitoredCmdableFactory to create
// monitored redis.Client objects.
func ExampleMonitoredCmdableFactory_client() {
	// variables should be properly initialized in production code
	var tracer *tracing.Tracer
	// Create a factory
	factory := redisbp.NewMonitoredClientFactory(
		"redis",
		redis.NewClient(&redis.Options{Addr: ":6379"}),
	)
	// Get a context object and a server Span, with the server Span set on the
	// context
	ctx, _ := tracing.CreateServerSpanForContext(context.Background(), tracer, "test")
	// Create a new client using the context for the Server Span
	client := factory.BuildClient(ctx)
	// Commands should now be wrapped using Client Spans
	client.Ping()
}

// This example demonstrates how to use a MonitoredCmdableFactory to create
// monitored redis.ClusterClient objects.
func ExampleMonitoredCmdableFactory_cluster() {
	// variables should be properly initialized in production code
	var tracer *tracing.Tracer
	// Create a factory
	factory := redisbp.NewMonitoredClusterFactory(
		"redis",
		redis.NewClusterClient(&redis.ClusterOptions{
			Addrs: []string{":7000", ":7001", ":7002"},
		}),
	)
	// Get a context object and a server Span, with the server Span set on the
	// context
	ctx, _ := tracing.CreateServerSpanForContext(context.Background(), tracer, "test")
	// Create a new client using the context for the Server Span
	client := factory.BuildClient(ctx)
	// Commands should now be wrapped using Client Spans
	client.Ping()
}

// This example demonstrates how to use a MonitoredCmdableFactory to create
// monitored redis.Client objects that implement failover using Redis Sentinel.
func ExampleMonitoredCmdableFactory_sentinel() {
	// variables should be properly initialized in production code
	var tracer *tracing.Tracer
	// Create a factory
	factory := redisbp.NewMonitoredClientFactory(
		"redis",
		redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:    "master",
			SentinelAddrs: []string{":6379"},
		}),
	)
	// Get a context object and a server Span, with the server Span set on the
	// context
	ctx, _ := tracing.CreateServerSpanForContext(context.Background(), tracer, "test")
	// Create a new client using the context for the Server Span
	client := factory.BuildClient(ctx)
	// Commands should now be wrapped using Client Spans
	client.Ping()
}

// This example demonstrates how to use a MonitoredCmdableFactory to create
// monitored redis.Ring objects.
func ExampleMonitoredCmdableFactory_ring() {
	// variables should be properly initialized in production code
	var tracer *tracing.Tracer
	// Create a factory
	factory := redisbp.NewMonitoredRingFactory(
		"redis",
		redis.NewRing(&redis.RingOptions{
			Addrs: map[string]string{
				"shard0": ":7000",
				"shard1": ":7001",
				"shard2": ":7002",
			},
		}),
	)
	// Get a context object and a server Span, with the server Span set on the
	// context
	ctx, _ := tracing.CreateServerSpanForContext(context.Background(), tracer, "test")
	// Create a new client using the context for the Server Span
	client := factory.BuildClient(ctx)
	// Commands should now be wrapped using Client Spans
	client.Ping()
}
