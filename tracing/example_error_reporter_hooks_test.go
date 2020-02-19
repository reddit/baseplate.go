package tracing_test

import (
	"errors"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/reddit/baseplate.go/tracing"
)

// This example demonstrates how to use ErrorReporterCreateServerSpanHook.
func ExampleErrorReporterCreateServerSpanHook() {
	// variables should be properly initialized in production code
	var (
		parent *tracing.Span
	)
	// initialize the ErrorReporterCreateServerSpanHook
	hook := tracing.ErrorReporterCreateServerSpanHook{}

	// register the hook with Baseplate
	tracing.RegisterCreateServerSpanHooks(hook)

	// Create a child server Span
	span := opentracing.StartSpan(
		"test",
		opentracing.ChildOf(parent),
		tracing.SpanTypeOption{Type: tracing.SpanTypeServer},
	)

	// Errors given to span.FinishWithOptions will be sent to Sentry
	span.FinishWithOptions(tracing.FinishOptions{
		Err: errors.New("test error"),
	}.Convert())
}
