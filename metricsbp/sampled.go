package metricsbp

import (
	"github.com/reddit/baseplate.go/randbp"

	"github.com/go-kit/kit/metrics"
)

// SampledCounter is a metrics.Counter implementation that actually sample the
// Add calls.
type SampledCounter struct {
	Counter metrics.Counter

	Rate float64
}

// With implements metrics.Counter.
func (c SampledCounter) With(tagValues ...string) metrics.Counter {
	return SampledCounter{
		Counter: c.Counter.With(tagValues...),
		Rate:    c.Rate,
	}
}

// Add implements metrics.Counter.
func (c SampledCounter) Add(delta float64) {
	if randbp.ShouldSampleWithRate(c.Rate) {
		c.Counter.Add(delta)
	}
}

// SampledHistogram is a metrics.Histogram implementation that actually sample
// the Observe calls.
type SampledHistogram struct {
	Histogram metrics.Histogram

	Rate float64
}

// With implements metrics.Histogram.
func (h SampledHistogram) With(labelValues ...string) metrics.Histogram {
	return SampledHistogram{
		Histogram: h.Histogram.With(labelValues...),
		Rate:      h.Rate,
	}
}

// Observe implements metrics.Histogram.
func (h SampledHistogram) Observe(value float64) {
	if randbp.ShouldSampleWithRate(h.Rate) {
		h.Histogram.Observe(value)
	}
}
