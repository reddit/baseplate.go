package redisbp_test

import (
	"context"

	"github.com/go-redis/redis/v8"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/reddit/baseplate.go/metricsbp"
	"github.com/reddit/baseplate.go/redis/deprecated/redisbp"
	"github.com/reddit/baseplate.go/tracing"
)

// Service is an example go-kit service to help demonstrate how to use
// redis.Cmdable in a service.
type Service struct {
	Redis redis.Cmdable
}

// Endpoint is an example endpoint that will use redis.
func (s Service) Endpoint(ctx context.Context) error {
	return EndpointHandler(ctx, s.Redis)
}

// EndpointHandler is the actual handler function for
// Service.Endpoint.
func EndpointHandler(ctx context.Context, client redis.Cmdable) error {
	// Any calls using the injected Redis client will be monitored using Spans.
	client.Ping(ctx)
	return nil
}

// This example demonstrates how to use a NewMonitoredClient.
func ExampleNewMonitoredClient() {
	name := "redis"
	// Create a basic, monitored redis.Client object.
	client := redisbp.NewMonitoredClient(name, &redis.Options{Addr: ":6379"})

	defer client.Close()

	// Spawn "MonitorPoolStats" in a separate goroutine.
	go redisbp.MonitorPoolStats(
		metricsbp.M.Ctx(),
		client,
		name,
		metricsbp.Tags{"env": "test"},
	)

	// Create a "service" with a monitored client.
	svc := Service{Redis: client}

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
