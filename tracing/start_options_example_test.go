package tracing_test

import (
	"context"

	"github.com/opentracing/opentracing-go"

	"github.com/reddit/baseplate.go/tracing"
)

// This example demonstrates how to use SpanTypeOption to create a client span.
func ExampleSpanTypeOption_client() {
	var (
		// In real code this should be in the args of your function
		ctx context.Context
		// In real code this should be the named return of your function
		err error
	)

	span, ctx := opentracing.StartSpanFromContext(
		ctx,
		"service.endpoint", // For example, "mysql.query"
		tracing.SpanTypeOption{
			Type: tracing.SpanTypeClient,
		},
	)
	// NOTE: It's important to wrap span.FinishWithOptions in a lambda.
	//
	// If you just do this:
	//
	//     // Bad example, DO NOT USE.
	//     defer span.FinishWithOptions(tracing.FinishOptions{
	//       Ctx: ctx,
	//       Err: err,
	//     }.Convert())
	//
	// Err will always be nil,
	// because the args of defer'd function are evaluated at the time of defer,
	// not time of execution.
	defer func() {
		span.FinishWithOptions(tracing.FinishOptions{
			Ctx: ctx,
			Err: err,
		}.Convert())
	}()

	// Do real work here.
}
