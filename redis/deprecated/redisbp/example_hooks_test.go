package redisbp_test

import (
	"context"

	"github.com/go-redis/redis/v8"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/reddit/baseplate.go/redis/deprecated/redisbp"
	"github.com/reddit/baseplate.go/tracing"
)

// This example demonstrates how to use SpanHook to automatically add Spans
// around Redis commands using go-redis
//
// Baseplate.go also provides the MonitoredCmdableFactory object that you can
// use to create Redis clients with a SpanHook already attached.
func ExampleSpanHook() {
	// variables should be properly initialized in production code
	var (
		// baseClient is not actually used to run commands, we register the Hook
		// to it and use it to create clients for each Server Span.
		baseClient redis.Client
	)
	// Add the Hook onto baseClient
	baseClient.AddHook(redisbp.SpanHook{ClientName: "redis"})
	// Create a server span and attach it into a context object
	_, ctx := opentracing.StartSpanFromContext(
		context.Background(),
		"test",
		tracing.SpanTypeOption{Type: tracing.SpanTypeServer},
	)
	// Create a new client using the context for the server span
	client := baseClient.WithContext(ctx)
	// Commands should now be wrapped using Client Spans
	client.Ping(ctx)
}
