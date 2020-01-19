package metricsbp_test

import (
	"context"

	"github.com/reddit/baseplate.go/metricsbp"
)

// This example demonstrates how to use Timer.
func ExampleTimer() {
	type timerContextKeyType struct{}
	// variables should be properly initialized in production code
	var (
		statsd          metricsbp.Statsd
		ctx             context.Context
		timerContextKey timerContextKeyType
	)
	const metricsPath = "dummy.call.timer"

	// initialize and inject a timer into context
	ctx = context.WithValue(
		ctx,
		timerContextKey,
		metricsbp.NewTimer(statsd.Histogram(metricsPath)),
	)
	// do the work
	dummyCall(ctx)
	// get the timer out of context and report
	if t, ok := ctx.Value(timerContextKey).(*metricsbp.Timer); ok {
		t.ObserveDuration()
	}
}

func dummyCall(_ context.Context) {}
