package promtest

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
)

// PrometheusMetricTest stores information about a metric to use for testing.
type PrometheusMetricTest struct {
	tb        testing.TB
	metric    prometheus.Collector
	name      string
	initValue float64
	initCount int
	labels    prometheus.Labels
}

// CheckDelta checks that the metric value changes exactly delta from when Helper was called.
func (p *PrometheusMetricTest) CheckDelta(delta float64) {
	p.tb.Helper()

	got, _ := p.getValue()
	got -= float64(p.initValue)
	if got != delta {
		p.tb.Errorf("%s metric delta: wanted %v, got %v", p.name, delta, got)
	}
}

// CheckDeltaLE checks that the metric value changes less or equal to delta from when Helper was called.
func (p *PrometheusMetricTest) CheckDeltaLE(delta float64) {
	p.tb.Helper()

	got, _ := p.getValue()
	got -= float64(p.initValue)
	if got > delta {
		p.tb.Errorf("%s metric delta: wanted less or equal to %v, got %v", p.name, delta, got)
	}
}

// CheckDeltaLT checks that the metric value changes less than delta from when Helper was called.
func (p *PrometheusMetricTest) CheckDeltaLT(delta float64) {
	p.tb.Helper()

	got, _ := p.getValue()
	got -= float64(p.initValue)
	if got >= delta {
		p.tb.Errorf("%s metric delta: wanted less than %v, got %v", p.name, delta, got)
	}
}

// CheckDeltaGE checks that the metric value changes greater or equal to delta from when Helper was called.
func (p *PrometheusMetricTest) CheckDeltaGE(delta float64) {
	p.tb.Helper()

	got, _ := p.getValue()
	got -= float64(p.initValue)
	if got < delta {
		p.tb.Errorf("%s metric delta: wanted greater or equal to %v, got %v", p.name, delta, got)
	}
}

// CheckDeltaGT checks that the metric value changes greater than delta from when Helper was called.
func (p *PrometheusMetricTest) CheckDeltaGT(delta float64) {
	p.tb.Helper()

	got, _ := p.getValue()
	got -= float64(p.initValue)
	if got <= delta {
		p.tb.Errorf("%s metric delta: wanted greater than %v, got %v", p.name, delta, got)
	}
}

// CheckDeltaHist checks that the histogram sum and count changes exactly the given delta
// from when Helper was called.
func (p *PrometheusMetricTest) CheckDeltaHist(deltaSum float64, deltaCount int) {
	p.tb.Helper()

	sum, count := p.getValue()
	sum -= float64(p.initValue)
	if sum != deltaSum {
		p.tb.Errorf("%s metric delta: wanted %v, got %v", p.name, deltaSum, sum)
	}
	count -= p.initCount
	if count != deltaCount {
		p.tb.Errorf("%s metric delta: wanted %v, got %v", p.name, deltaCount, count)
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
	p := &PrometheusMetricTest{
		tb:     tb,
		metric: metric,
		name:   name,
		labels: labels,
	}
	p.initValue, p.initCount = p.getValue()
	return p
}

// getValue returns the current value of the metric.
func (p *PrometheusMetricTest) getValue() (value float64, count int) {
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
			p.tb.Fatalf("histogram is not a collector type")
		}
		count, value = collectHistogramToFloat64(p.tb, h)
	case prometheus.Histogram:
		count, value = collectHistogramToFloat64(p.tb, m)
	default:
		p.tb.Fatalf("not supported type %T for metric %s\n", m, p.name)
	}
	return
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
