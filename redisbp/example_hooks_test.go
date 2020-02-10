package redisbp_test

import (
	"context"

	"github.com/go-redis/redis/v7"

	"github.com/reddit/baseplate.go/redisbp"
	"github.com/reddit/baseplate.go/tracing"
)

// This example demonstrates how to use SpanHook to automatically add Spans
// around Redis commands using go-redis
//
// baseplate.go also provides the MonitoredCmdableFactory object that you can
// use to create Redis clients with a SpanHook already attached.
func ExampleSpanHook() {
	// variables should be properly initialized in production code
	var (
		// baseClient is not actually used to run commands, we register the Hook
		// to it and use it to create clients for each Server Span.
		baseClient redis.Client
		tracer     *tracing.Tracer
	)
	// Add the Hook onto baseClient
	baseClient.AddHook(redisbp.SpanHook{ClientName: "redis"})
	// Get a context object and a server Span, with the server Span set on the
	// context
	ctx, _ := tracing.CreateServerSpanForContext(context.Background(), tracer, "test")
	// Create a new client using the context for the Server Span
	client := baseClient.WithContext(ctx)
	// Commands should now be wrapped using Client Spans
	client.Ping()
}
