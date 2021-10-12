package metricsbp

import (
	"fmt"
	"strings"
	"time"

	"github.com/reddit/baseplate.go/tracing"
)

const (
	baseplateTimer   = "baseplate.%s.latency"
	baseplateCounter = "baseplate.%s.rate"
)

// CreateServerSpanHook registers each Server Span with a MetricsSpanHook.
type CreateServerSpanHook struct {
	// Optional, will fallback to M when it's nil.
	Metrics *Statsd

	PrometheusMetrics *PrometheusMetrics
}

// OnCreateServerSpan registers MetricSpanHooks on a server Span.
func (h CreateServerSpanHook) OnCreateServerSpan(span *tracing.Span) error {
	statsd := h.Metrics.fallback()
	span.AddHooks(
		newSpanHook(statsd, h.PrometheusMetrics, span),
		countActiveRequestsHook{
			metrics:           statsd,
			prometheusMetrics: h.PrometheusMetrics,
		},
	)
	return nil
}

// spanHook wraps a Span in a timer and records a counter metric when the Span
// ends, with success=True/False tags for the counter based on whether an error
// was passed to `span.End` or not.
type spanHook struct {
	metrics           *Statsd
	prometheusMetrics *PrometheusMetrics

	startTime time.Time
}

func newSpanHook(metrics *Statsd, pm *PrometheusMetrics, span *tracing.Span) *spanHook {
	return &spanHook{
		metrics:           metrics,
		prometheusMetrics: pm,
	}
}

// OnCreateChild registers a child MetricsSpanHook on the child Span and starts
// a new Timer around the Span.
func (h *spanHook) OnCreateChild(parent, child *tracing.Span) error {
	child.AddHooks(newSpanHook(h.metrics, h.prometheusMetrics, child))
	return nil
}

func splitClientSpanName(name string) (client, endpoint string) {
	index := strings.LastIndexByte(name, '.')
	if index < 0 {
		// No client name
		return "", name
	}
	return name[:index], name[index+1:]
}

// OnPostStart records the start time for the timer,
// and also sets the endpoint and client tags for the span.
func (h *spanHook) OnPostStart(span *tracing.Span) error {
	if span.SpanType() == tracing.SpanTypeClient {
		client, endpoint := splitClientSpanName(span.Name())
		span.SetTag(tracing.TagKeyClient, client)
		span.SetTag(tracing.TagKeyEndpoint, endpoint)
	} else {
		span.SetTag(tracing.TagKeyEndpoint, span.Name())
	}

	if span.StartTime().IsZero() {
		h.startTime = time.Now()
	} else {
		h.startTime = span.StartTime()
	}
	return nil
}

// OnPreStop stops the Timer started in OnPostStart and records a metric
// indicating if the span was a "success" or "failure".
//
// A span is marked as "failure" if `err != nil` otherwise it is marked as
// "success".
func (h *spanHook) OnPreStop(span *tracing.Span, err error) error {
	stop := span.StopTime()
	if stop.IsZero() {
		stop = time.Now()
	}
	typeStr := span.SpanType().String()
	tags := Tags(span.MetricsTags())
	timer := NewTimer(h.metrics.Timing(
		fmt.Sprintf(baseplateTimer, typeStr),
	).With(tags.AsStatsdTags()...))
	timer.OverrideStartTime(h.startTime).ObserveWithEndTime(stop)

	var successResult string
	if err != nil {
		successResult = "False"
	} else {
		successResult = "True"
	}
	tags[tracing.TagKeySuccess] = successResult
	h.metrics.Counter(fmt.Sprintf(baseplateCounter, typeStr)).With(tags.AsStatsdTags()...).Add(1)

	return nil
}

// OnAddCounter will increment a metric by "delta" using "key" as the metric
// "name"
func (h *spanHook) OnAddCounter(span *tracing.Span, key string, delta float64) error {
	h.metrics.Counter(key).With(Tags(span.MetricsTags()).AsStatsdTags()...).Add(delta)
	return nil
}

type countActiveRequestsHook struct {
	metrics           *Statsd
	prometheusMetrics *PrometheusMetrics
}

func (h countActiveRequestsHook) OnPostStart(_ *tracing.Span) error {
	h.metrics.incActiveRequests()
	h.prometheusMetrics.incActiveRequests()
	return nil
}

func (h countActiveRequestsHook) OnPreStop(_ *tracing.Span, _ error) error {
	h.metrics.decActiveRequests()
	h.prometheusMetrics.decActiveRequests()
	return nil
}

var (
	_ tracing.CreateServerSpanHook = CreateServerSpanHook{}
	_ tracing.StartStopSpanHook    = (*spanHook)(nil)
	_ tracing.CreateChildSpanHook  = (*spanHook)(nil)
	_ tracing.AddSpanCounterHook   = (*spanHook)(nil)
	_ tracing.StartStopSpanHook    = countActiveRequestsHook{}
)
