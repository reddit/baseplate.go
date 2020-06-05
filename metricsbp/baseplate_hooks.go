package metricsbp

import (
	"fmt"
	"time"

	"github.com/go-kit/kit/metrics"
	"github.com/reddit/baseplate.go/tracing"
)

const (
	success = "success"
	fail    = "fail"
	failure = "failure"
	total   = "total"
)

// CreateServerSpanHook registers each Server Span with a MetricsSpanHook.
type CreateServerSpanHook struct {
	// Optional, will fallback to M when it's nil.
	Metrics *Statsd
}

// OnCreateServerSpan registers MetricSpanHooks on a server Span.
func (h CreateServerSpanHook) OnCreateServerSpan(span *tracing.Span) error {
	span.AddHooks(newSpanHook(h.Metrics.fallback(), span))
	return nil
}

// spanHook wraps a Span in a Timer and records a "success" or "fail"/"failure"
// metric when the Span ends based on whether an error was passed to `span.End`
// or not.
type spanHook struct {
	name    string
	metrics *Statsd

	histogram metrics.Histogram
	start     time.Time
}

func newSpanHook(metrics *Statsd, span *tracing.Span) *spanHook {
	name := span.Component() + "." + span.Name()
	return &spanHook{
		name:      name,
		metrics:   metrics,
		histogram: metrics.Timing(name),
	}
}

// OnCreateChild registers a child MetricsSpanHook on the child Span and starts
// a new Timer around the Span.
func (h *spanHook) OnCreateChild(parent, child *tracing.Span) error {
	child.AddHooks(newSpanHook(h.metrics, child))
	return nil
}

// OnPostStart starts the timer.
func (h *spanHook) OnPostStart(span *tracing.Span) error {
	if span.StartTime().IsZero() {
		h.start = time.Now()
	} else {
		h.start = span.StartTime()
	}
	return nil
}

// OnPreStop stops the Timer started in OnPostStart and records a metric
// indicating if the span was a "success" or "fail".
//
// A span is marked as "fail" if `err != nil` otherwise it is marked as
// "success".
func (h *spanHook) OnPreStop(span *tracing.Span, err error) error {
	var duration time.Duration
	if span.StopTime().IsZero() {
		duration = time.Since(h.start)
	} else {
		duration = span.StopTime().Sub(h.start)
	}
	recordDuration(h.histogram, duration)

	var statusMetricPath string
	if err != nil {
		statusMetricPath = fmt.Sprintf("%s.%s", h.name, failure)
		// temp: publish both "fail" and "failure"
		h.metrics.Counter(fmt.Sprintf("%s.%s", h.name, fail)).Add(1)
	} else {
		statusMetricPath = fmt.Sprintf("%s.%s", h.name, success)
	}
	h.metrics.Counter(statusMetricPath).Add(1)
	h.metrics.Counter(fmt.Sprintf("%s.%s", h.name, total)).Add(1)
	return nil
}

// OnAddCounter will increment a metric by "delta" using "key" as the metric
// "name"
func (h *spanHook) OnAddCounter(span *tracing.Span, key string, delta float64) error {
	h.metrics.Counter(key).Add(delta)
	return nil
}

var (
	_ tracing.CreateServerSpanHook = CreateServerSpanHook{}
	_ tracing.StartStopSpanHook    = (*spanHook)(nil)
	_ tracing.CreateChildSpanHook  = (*spanHook)(nil)
	_ tracing.AddSpanCounterHook   = (*spanHook)(nil)
)
