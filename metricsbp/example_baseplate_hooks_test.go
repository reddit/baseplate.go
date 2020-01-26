package metricsbp_test

import (
	"github.com/reddit/baseplate.go/metricsbp"
	"github.com/reddit/baseplate.go/tracing"
)

// This example demonstrates how to use BaseplateHook.
func ExampleBaseplateHook() {
	const prefix = "service.server"

	// initialize the BaseplateHook
	hook := metricsbp.BaseplateHook{}

	// register the hook with Baseplate
	tracing.RegisterBaseplateHook(hook)
}
