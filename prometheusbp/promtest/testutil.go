package promtest

import (
	"math"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
)

const float64EqualityThreshold = 1e-9

func almostEqual(a, b float64) bool {
	if math.IsNaN(a) {
		return math.IsNaN(b)
	}
	if math.IsInf(a, 0) {
		aSign := 1
		if a < 0 {
			aSign = -1
		}
		return math.IsInf(b, aSign)
	}
	threshold := math.Abs(a) * float64EqualityThreshold
	return math.Abs(a-b) <= threshold
}

// PrometheusMetricTest stores information about a metric to use for testing.
type PrometheusMetricTest struct {
	tb              testing.TB
	metric          prometheus.Collector
	name            string
	initValue       float64
	initSampleCount int
	labels          prometheus.Labels
}

// CheckDelta checks that the metric value changes exactly delta from when Helper was called.
func (p *PrometheusMetricTest) CheckDelta(delta float64) {
	p.tb.Helper()

	got, _ := p.getValueAndSampleCount()
	got -= p.initValue
	if !almostEqual(got, delta) {
		p.tb.Errorf("%s metric delta: wanted %v, got %v", p.name, delta, got)
	}
}

// CheckSampleCountDelta checks that the number of samples (of histogram) is the exactly delta from when Helper was called.
func (p *PrometheusMetricTest) CheckSampleCountDelta(delta int) {
	p.tb.Helper()

	_, got := p.getValueAndSampleCount()
	got -= p.initSampleCount
	if got != delta {
		p.tb.Errorf("%s metric histogram count delta: wanted %v, got %v", p.name, delta, got)
	}
}

// CheckExists confirms that the metric exists and returns exactly 1 metrics.
//
// It's a shorthand for CheckExistsN(1)
func (p *PrometheusMetricTest) CheckExists() {
	p.tb.Helper()
	p.CheckExistsN(1)
}

// CheckExistsN confirms that the metric exists and returns the count of metrics.
//
// Please note that due to the limitation of upstream API,
// neither CheckExistsN nor CheckExists will limit the counts to the specified
// labels, so they will check against the number of metrics reported with all
// label values.
func (p *PrometheusMetricTest) CheckExistsN(count int) {
	p.tb.Helper()

	got := testutil.CollectAndCount(p.metric)
	if got != count {
		p.tb.Errorf("%s metric count: wanted %v, got %v", p.name, count, got)
	}
}

// NewPrometheusMetricTest creates a new test object for a Prometheus metric.
// It stores the current value of the metric along with the metric name.
func NewPrometheusMetricTest(tb testing.TB, name string, metric prometheus.Collector, labels prometheus.Labels) *PrometheusMetricTest {
	tb.Helper()
	p := &PrometheusMetricTest{
		tb:     tb,
		metric: metric,
		name:   name,
		labels: labels,
	}
	p.initValue, p.initSampleCount = p.getValueAndSampleCount()
	return p
}

// getValueAndSampleCount returns the current value and histogram sample count
// of the metric.
func (p *PrometheusMetricTest) getValueAndSampleCount() (float64, int) {
	p.tb.Helper()
	var value float64
	var histoCount int
	switch m := p.metric.(type) {
	case prometheus.Gauge, prometheus.Counter:
		value = testutil.ToFloat64(m)
	case *prometheus.GaugeVec:
		gague, err := m.GetMetricWith(p.labels)
		if err != nil {
			p.tb.Fatalf("get %s metric err %v", p.name, err)
		}
		value = testutil.ToFloat64(gague)
	case *prometheus.CounterVec:
		counter, err := m.GetMetricWith(p.labels)
		if err != nil {
			p.tb.Fatalf("get %s metric err %v", p.name, err)
		}
		value = testutil.ToFloat64(counter)
	case *prometheus.HistogramVec:
		histrogram, err := m.GetMetricWith(p.labels)
		if err != nil {
			p.tb.Fatalf("get %s metric err %v", p.name, err)
		}
		h, ok := histrogram.(prometheus.Collector)
		if !ok {
			p.tb.Fatalf("histogram %s is not a collector type", p.name)
		}
		histoCount, value = collectHistogramToFloat64(p.tb, h)
	case prometheus.Histogram:
		histoCount, value = collectHistogramToFloat64(p.tb, m)
	default:
		p.tb.Fatalf("not supported type %T for metric %s", m, p.name)
	}
	return value, histoCount
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
