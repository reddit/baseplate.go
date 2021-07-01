package metricsbp_test

import (
	"github.com/reddit/baseplate.go/metricsbp"
	"github.com/reddit/baseplate.go/tracing"
)

// This example demonstrates how to use CreateServerSpanHook.
func ExampleCreateServerSpanHook() {
	// initialize the CreateServerSpanHook
	hook := metricsbp.CreateServerSpanHook{}

	// register the hook with Baseplate
	tracing.RegisterCreateServerSpanHooks(hook)
}
