package redisbp_test

import (
	"context"

	"github.com/go-redis/redis/v7"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/reddit/baseplate.go/metricsbp"
	"github.com/reddit/baseplate.go/redis/deprecated/redisbp"
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
	// Create a factory for basic, monitored redis.Client objects.
	clientFactory := redisbp.NewMonitoredClientFactory(
		"redis",
		redis.NewClient(&redis.Options{Addr: ":6379"}),
	)
	// Create a factory for monitored redis.Client objects that implements
	// failover with Redis Sentinel.
	failoverFactory := redisbp.NewMonitoredClientFactory(
		"redis",
		redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:    "master",
			SentinelAddrs: []string{":6379"},
		}),
	)
	// Create a factory for monitored redis.ClusterClient objects.
	clusterFactory := redisbp.NewMonitoredClusterFactory(
		"redis",
		redis.NewClusterClient(&redis.ClusterOptions{
			Addrs: []string{":7000", ":7001", ":7002"},
		}),
	)
	defer func() {
		clientFactory.Close()
		failoverFactory.Close()
		clusterFactory.Close()
	}()

	go clientFactory.MonitorPoolStats(metricsbp.M.Ctx(), metricsbp.Tags{"env": "test"})

	// Create a "service" with a monitored client factory.
	svc := Service{RedisFactory: clientFactory}
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
}
