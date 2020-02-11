package redisbp_test

import (
	"context"

	"github.com/go-redis/redis/v7"

	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/redisbp"
	"github.com/reddit/baseplate.go/tracing"
)

// Service is an example go-kit service to help demonstrate how to use
// redisbp.type MonitoredCmdableFactory in a service.
type Service struct {
	RedisFactory redisbp.MonitoredCmdableFactory
}

// Endpoint is an example endpoint that will use redis.
func (s Service) Endpoint(ctx context.Context) error {
	// Use the factory to create a new, monitored redis client that can be
	// injected into your endpoint handler
	redis := s.RedisFactory.BuildClient(ctx)
	return EndpointHandler(ctx, redis)
}

// EndpointHandler is the actual handler function for
// Service.Endpoint.
func EndpointHandler(ctx context.Context, client redisbp.MonitoredCmdable) error {
	if span := tracing.GetServerSpan(ctx); span != nil {
		log.Debug("Span: %s", span.Name())
	}
	// Any calls using the injected Redis client will be monitored using Spans.
	client.Ping()
	return nil
}

// This example demonstrates how to use a MonitoredCmdableFactory to create
// monitored redis.Client objects.
func ExampleMonitoredCmdableFactory_client() {
	// variables should be properly initialized in production code
	var tracer *tracing.Tracer
	// Create a service with a factory
	svc := Service{
		RedisFactory: redisbp.NewMonitoredClientFactory(
			"redis",
			redis.NewClient(&redis.Options{Addr: ":6379"}),
		),
	}
	// Get a context object and a server Span, with the server Span set on the
	// context.
	//
	// In production, this will be handled by service middleware rather than
	// being called manually.
	ctx, _ := tracing.CreateServerSpanForContext(context.Background(), tracer, "test")
	// Calls to this endpoint will use the factory to create a new client and
	// inject it into the actual implementation
	//
	// In production, the service framework will call these endpoints in
	// response to requests from clients rather than you calling it manually.
	svc.Endpoint(ctx)
}

// This example demonstrates how to use a MonitoredCmdableFactory to create
// monitored redis.ClusterClient objects.
func ExampleMonitoredCmdableFactory_cluster() {
	// variables should be properly initialized in production code
	var tracer *tracing.Tracer
	// Create service with a factory
	svc := Service{
		RedisFactory: redisbp.NewMonitoredClusterFactory(
			"redis",
			redis.NewClusterClient(&redis.ClusterOptions{
				Addrs: []string{":7000", ":7001", ":7002"},
			}),
		),
	}
	// Get a context object and a server Span, with the server Span set on the
	// context
	//
	// In production, this will be handled by service middleware rather than
	// being called manually.
	ctx, _ := tracing.CreateServerSpanForContext(context.Background(), tracer, "test")
	// Calls to this endpoint will use the factory to create a new client and
	// inject it into the actual implementation
	//
	// In production, the service framework will call these endpoints in
	// response to requests from clients rather than you calling it manually.
	svc.Endpoint(ctx)
}

// This example demonstrates how to use a MonitoredCmdableFactory to create
// monitored redis.Client objects that implement failover using Redis Sentinel.
func ExampleMonitoredCmdableFactory_sentinel() {
	// variables should be properly initialized in production code
	var tracer *tracing.Tracer
	// Create service with a factory
	svc := Service{
		RedisFactory: redisbp.NewMonitoredClientFactory(
			"redis",
			redis.NewFailoverClient(&redis.FailoverOptions{
				MasterName:    "master",
				SentinelAddrs: []string{":6379"},
			}),
		),
	}
	// Get a context object and a server Span, with the server Span set on the
	// context
	//
	// In production, this will be handled by service middleware rather than
	// being called manually.
	ctx, _ := tracing.CreateServerSpanForContext(context.Background(), tracer, "test")
	// Calls to this endpoint will use the factory to create a new client and
	// inject it into the actual implementation
	//
	// In production, the service framework will call these endpoints in
	// response to requests from clients rather than you calling it manually.
	svc.Endpoint(ctx)
}

// This example demonstrates how to use a MonitoredCmdableFactory to create
// monitored redis.Ring objects.
func ExampleMonitoredCmdableFactory_ring() {
	// variables should be properly initialized in production code
	var tracer *tracing.Tracer
	// Create service with a factory
	svc := Service{
		RedisFactory: redisbp.NewMonitoredRingFactory(
			"redis",
			redis.NewRing(&redis.RingOptions{
				Addrs: map[string]string{
					"shard0": ":7000",
					"shard1": ":7001",
					"shard2": ":7002",
				},
			}),
		),
	}
	// Get a context object and a server Span, with the server Span set on the
	// context
	//
	// In production, this will be handled by service middleware rather than
	// being called manually.
	ctx, _ := tracing.CreateServerSpanForContext(context.Background(), tracer, "test")
	// Calls to this endpoint will use the factory to create a new client and
	// inject it into the actual implementation
	//
	// In production, the service framework will call these endpoints in
	// response to requests from clients rather than you calling it manually.
	svc.Endpoint(ctx)
}
