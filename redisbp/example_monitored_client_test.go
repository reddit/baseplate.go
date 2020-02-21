package redisbp_test

import (
	"context"

	"github.com/go-redis/redis/v7"
	opentracing "github.com/opentracing/opentracing-go"
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
func EndpointHandler(_ context.Context, client redisbp.MonitoredCmdable) error {
	// Any calls using the injected Redis client will be monitored using Spans.
	client.Ping()
	return nil
}

// This example demonstrates how to use a MonitoredCmdableFactory.
func ExampleMonitoredCmdableFactory() {
	// Create a service with a factory using a basic redis.Client
	svc := Service{
		RedisFactory: redisbp.NewMonitoredClientFactory(
			"redis",
			redis.NewClient(&redis.Options{Addr: ":6379"}),
		),
	}
	// Create a server span and attach it to a context object.
	//
	// In production, this will be handled by service middleware rather than
	// being called manually.
	_, ctx := opentracing.StartSpanFromContext(
		context.Background(),
		"test",
		tracing.SpanTypeOption{Type: tracing.SpanTypeServer},
	)
	// Calls to this endpoint will use the factory to create a new client and
	// inject it into the actual implementation
	//
	// In production, the service framework will call these endpoints in
	// response to requests from clients rather than you calling it manually.
	svc.Endpoint(ctx)

	// Create service with a factory using a redis.Client that implements failover
	// with Redis Sentinel.
	svc = Service{
		RedisFactory: redisbp.NewMonitoredClientFactory(
			"redis",
			redis.NewFailoverClient(&redis.FailoverOptions{
				MasterName:    "master",
				SentinelAddrs: []string{":6379"},
			}),
		),
	}
	_, ctx = opentracing.StartSpanFromContext(
		context.Background(),
		"test",
		tracing.SpanTypeOption{Type: tracing.SpanTypeServer},
	)
	svc.Endpoint(ctx)

	// Create service with a factory using a redis.ClusterClient to connect to
	// Redis Cluster.
	svc = Service{
		RedisFactory: redisbp.NewMonitoredClusterFactory(
			"redis",
			redis.NewClusterClient(&redis.ClusterOptions{
				Addrs: []string{":7000", ":7001", ":7002"},
			}),
		),
	}
	_, ctx = opentracing.StartSpanFromContext(
		context.Background(),
		"test",
		tracing.SpanTypeOption{Type: tracing.SpanTypeServer},
	)
	svc.Endpoint(ctx)

	// Create service with a factory using a redis.Ring client.
	svc = Service{
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
	_, ctx = opentracing.StartSpanFromContext(
		context.Background(),
		"test",
		tracing.SpanTypeOption{Type: tracing.SpanTypeServer},
	)
	svc.Endpoint(ctx)
}
