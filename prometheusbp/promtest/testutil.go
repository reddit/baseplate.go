package promtest

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// PrometheusMetricTest stores information about a metric to use for testing.
type PrometheusMetricTest struct {
	tb          testing.TB
	metric      prometheus.Collector
	name        string
	initValue   float64
	labelValues []string
}

// CheckDelta checks that the metric value changes exactly delta from when Helper was called.
func (p *PrometheusMetricTest) CheckDelta(delta float64) {
	p.tb.Helper()
	got := p.getValue()
	got -= float64(p.initValue)
	if got != delta {
		p.tb.Errorf("%s metric delta: wanted %v, got %v", p.name, delta, got)
	}
}

// CheckExists confirms that the metric exists and returns the count of metrics.
func (p *PrometheusMetricTest) CheckExists() {
	p.tb.Helper()
	got := testutil.CollectAndCount(p.metric)
	if got != 1 {
		p.tb.Errorf("%s metric count: wanted %v, got %v", p.name, 1, got)
	}
}

// NewPrometheusMetricTest creates a new test object for a Prometheus metric.
// It stores the current value of the metric along with the metric name.
func NewPrometheusMetricTest(tb testing.TB, name string, metric prometheus.Collector, labelValues ...string) *PrometheusMetricTest {
	p := &PrometheusMetricTest{
		tb:          tb,
		metric:      metric,
		name:        name,
		labelValues: labelValues,
	}
	p.initValue = p.getValue()
	return p
}

// getValue returns the current value of the metric.
func (p *PrometheusMetricTest) getValue() float64 {
	var value float64
	switch m := p.metric.(type) {
	case *prometheus.GaugeVec:
		gague, err := m.GetMetricWithLabelValues(p.labelValues...)
		if err != nil {
			p.tb.Fatalf("get %s metric err %v", p.name, err)
		}
		value = testutil.ToFloat64(gague)
	case *prometheus.CounterVec:
		counter, err := m.GetMetricWithLabelValues(p.labelValues...)
		if err != nil {
			p.tb.Fatalf("get %s metric err %v", p.name, err)
		}
		value = testutil.ToFloat64(counter)
	case *prometheus.HistogramVec:
		// prometheus.HistogramVec not supported for testutil.ToFloat64
	default:
		p.tb.Fatalf("not supported type %T\n", m)
	}
	return value
}
