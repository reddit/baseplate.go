package prometheusbp

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
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
	got := p.getValue()
	got -= float64(p.initValue)
	if got != delta {
		p.tb.Errorf("%s metric delta: wanted %v, got %v", p.name, delta, got)
	}
}

// CheckExists confirms that the metric exists and returns the count of metrics.
func (p *PrometheusMetricTest) CheckExists() {
	got := testutil.CollectAndCount(p.metric)
	if got != 1 {
		p.tb.Errorf("%s metric count: wanted %v, got %v", p.name, 1, got)
	}
}

// CheckNotExists confirms that the metric does not exists.
func (p *PrometheusMetricTest) CheckNotExists() {
	got := testutil.CollectAndCount(p.metric)
	if got != 0 {
		p.tb.Errorf("%s metric count: wanted %v, got %v", p.name, 0, got)
	}
}

// MetricTest stores the current value of the metric along with the metric name
// to be used later for testing.
func MetricTest(tb testing.TB, name string, metric prometheus.Collector, labelValues ...string) *PrometheusMetricTest {
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
		histrogram, err := m.GetMetricWithLabelValues(p.labelValues...)
		if err != nil {
			p.tb.Fatalf("get %s metric err %v", p.name, err)
		}
		h, ok := histrogram.(prometheus.Collector)
		if !ok {
			p.tb.Fatalf("histogram is not a collector type")
		}
		_, value = collectHistogramToFloat64(p.tb, h)
	default:
		p.tb.Fatalf("not supported type %T\n", m)
	}
	return value
}

// collectHistogramToFloat64 returns the sample count and sum of the histogram.
func collectHistogramToFloat64(tb testing.TB, c prometheus.Collector) (int, float64) {
	var (
		m      prometheus.Metric
		mCount int
		mChan  = make(chan prometheus.Metric)
		done   = make(chan struct{})
	)

	go func() {
		for m = range mChan {
			mCount++
		}
		close(done)
	}()

	c.Collect(mChan)
	close(mChan)
	<-done

	if mCount != 1 {
		tb.Errorf("collected %d metrics instead of exactly 1", mCount)
	}

	pb := &dto.Metric{}
	m.Write(pb)
	if pb.Histogram != nil {
		return int(pb.GetHistogram().GetSampleCount()), pb.GetHistogram().GetSampleSum()
	}
	tb.Errorf("collected a non-histogram type metric: %s", pb)
	return 0, 0
}
