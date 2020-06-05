package metricsbp_test

import (
	"github.com/reddit/baseplate.go/errorsbp"
	"github.com/reddit/baseplate.go/metricsbp"
	"github.com/reddit/baseplate.go/tracing"
)

// This example demonstrates how to use CreateServerSpanHook.
func ExampleCreateServerSpanHook() {
	const prefix = "service.server"

	// initialize the CreateServerSpanHook
	hook := metricsbp.CreateServerSpanHook{
		Suppressor: errorsbp.NewSuppressor(),
	}

	// register the hook with Baseplate
	tracing.RegisterCreateServerSpanHooks(hook)
}
