package metricsbp_test

import (
	"context"
	"fmt"

	"github.com/reddit/baseplate.go/metricsbp"

	"github.com/go-kit/kit/metrics"
)

type SubMetrics struct {
	MyHistogram metrics.Histogram
	MyGauge     metrics.Gauge
}

type PreCreatedMetrics struct {
	MyCounter    metrics.Counter
	MySubMetrics SubMetrics
}

// This example demonstrate how to use CheckNilFields in your microservice code
// to pre-create frequently used metrics.
func ExampleCheckNilFields() {
	// In reality these should come from flag or other configurations.
	const (
		prefix     = "myservice"
		statsdAddr = "localhost:1234"
		sampleRate = 1
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	st := metricsbp.NewStatsd(
		ctx,
		metricsbp.StatsdConfig{
			Prefix:            prefix,
			Address:           statsdAddr,
			DefaultSampleRate: sampleRate,
		},
	)

	// Initialize metrics
	m := PreCreatedMetrics{
		MyCounter: st.Counter("my.counter"),
		MySubMetrics: SubMetrics{
			MyHistogram: st.Histogram("my.histogram"),
			// Forgot to initialize MyGauge here
		},
	}
	missingFields := metricsbp.CheckNilFields(m)
	if len(missingFields) > 0 {
		panic(fmt.Sprintf("Uninitialized metrics: %v", missingFields))
	}

	// Other initializations.

	// Replace with your actual service starter
	startService := func(m PreCreatedMetrics /* and other args */) {}

	startService(
		m,
		// other args
	)
}
