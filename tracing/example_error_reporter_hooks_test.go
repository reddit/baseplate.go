package tracing_test

import (
	"context"
	"errors"

	"github.com/reddit/baseplate.go/tracing"
)

// This example demonstrates how to use ErrorReporterBaseplateHook.
func ExampleErrorReporterBaseplateHook() {
	// variables should be properly initialized in production code
	var tracer *tracing.Tracer

	// initialize the ErrorReporterBaseplateHook
	hook := tracing.ErrorReporterBaseplateHook{}

	// register the hook with Baseplate
	tracing.RegisterBaseplateHook(hook)

	// Create a Span
	span := tracing.CreateServerSpan(tracer, "test")

	// Errors given to span.End will be sent to Sentry
	span.End(context.Background(), errors.New("test error"))
}
