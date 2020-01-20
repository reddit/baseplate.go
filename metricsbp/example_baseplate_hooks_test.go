package metricsbp_test

import (
	"github.com/reddit/baseplate.go/metricsbp"
	"github.com/reddit/baseplate.go/tracing"
)

// This example demonstrates how to use BaseplateHook.
func ExampleBaseplateHook() {
	// variables should be properly initialized in production code
	var statsd metricsbp.Statsd
	const prefix = "service.server"

	// initialize the BaseplateHook
	hook := metricsbp.BaseplateHook{
		Prefix:  prefix,
		Metrics: statsd,
	}

	// register the hook with Baseplate
	tracing.RegisterBaseplateHook(hook)
}
