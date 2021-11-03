package prometheusbp

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// PrometheusMetricTest stores information about a metric to use for testing.
type PrometheusMetricTest struct {
	t         *testing.T
	metric    prometheus.Collector
	name      string
	initValue int
}

// CheckDelta checks that the metric value changes exactly delta from when Helper was called.
func (o *PrometheusMetricTest) CheckDelta(delta float64, labelValues ...string) {
	var got float64
	switch m := o.metric.(type) {
	case *prometheus.GaugeVec:
		gague, err := m.GetMetricWithLabelValues(labelValues...)
		if err != nil {
			o.t.Fatalf("get %s metric err %v", o.name, err)
		}
		got = testutil.ToFloat64(gague)
	case *prometheus.CounterVec:
		counter, err := m.GetMetricWithLabelValues(labelValues...)
		if err != nil {
			o.t.Fatalf("get %s metric err %v", o.name, err)
		}
		got = testutil.ToFloat64(counter)
	default:
		o.t.Fatalf("not supported type %T\n", m)
	}
	got -= float64(o.initValue)
	if got != delta {
		o.t.Errorf("%s metric delta: wanted %v, got %v", o.name, delta, got)
	}
}

// MetricTest stores the current value of the metric along with the metric name
// to be used later for testing.
func MetricTest(t *testing.T, name string, metric prometheus.Collector) *PrometheusMetricTest {
	initVal := testutil.CollectAndCount(metric)
	return &PrometheusMetricTest{
		t:         t,
		metric:    metric,
		name:      name,
		initValue: initVal,
	}
}
