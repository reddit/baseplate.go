package integrations_test

import (
	"context"

	"github.com/go-redis/redis/v7"

	"github.com/reddit/baseplate.go/integrations"
	"github.com/reddit/baseplate.go/tracing"
)

// This example demonstrates how to use RedisSpanHook to automatically add Spans
// around Redis commands using go-redis
func ExampleRedisSpanHook() {
	// variables should be properly initialized in production code
	var (
		// baseClient is not actually used to run commands, we register the Hook
		// to it and use it to create clients for each Server Span.
		baseClient redis.Client
		span       *tracing.Span
	)
	// Add the Hook onto baseClient
	baseClient.AddHook(integrations.RedisSpanHook{ClientName: "redis"})
	// Create a context and attach a Server Span
	ctx, _ := span.SetServerSpan(context.Background())
	// Create a new client using the context for the Server Span
	client := baseClient.WithContext(ctx)
	// Commands should now be wrapped using Client Spans
	client.Ping()
}
