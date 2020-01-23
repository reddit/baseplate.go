package tracing_test

import (
	"context"
	"errors"

	"github.com/reddit/baseplate.go/tracing"
)

// This example demonstrates how to use ErrorReporterCreateServerSpanHook.
func ExampleErrorReporterCreateServerSpanHook() {
	// variables should be properly initialized in production code
	var (
		tracer *tracing.Tracer
		ctx    context.Context
	)

	// initialize the ErrorReporterCreateServerSpanHook
	hook := tracing.ErrorReporterCreateServerSpanHook{}

	// register the hook with Baseplate
	tracing.RegisterCreateServerSpanHook(hook)

	// Create a ServerSpan
	span := tracing.CreateServerSpan(tracer, "test")

	// Errors given to span.End will be sent to Sentry
	span.Stop(ctx, errors.New("test error"))
}
