package metricsbp_test

import (
	"testing"
	"time"

	"github.com/reddit/baseplate.go/metricsbp"

	"github.com/go-kit/kit/metrics"
)

type mockHistogram struct {
	t        *testing.T
	expected float64
	called   bool
}

func (m *mockHistogram) With(_ ...string) metrics.Histogram { return m }

func (m *mockHistogram) Observe(v float64) {
	m.called = true
	m.t.Logf("Expected reporting value %v, got %v", m.expected, v)
	// We just need to make sure that it's the correct unit,
	// and allow some actual time differences.
	if v > m.expected*2 || v < m.expected/10 {
		m.t.Fail()
	}
}

func TestTimer(t *testing.T) {
	const sleep = time.Millisecond
	h := mockHistogram{
		t:        t,
		expected: 1,
	}
	timer := metricsbp.NewTimer(&h)
	time.Sleep(sleep)
	timer.ObserveDuration()
	if !h.called {
		t.Errorf("histogram.Observe not called!")
	}
}

func TestTimerZero(_ *testing.T) {
	// Just make sure the code doesn't panic here, no actual tests.
	var t1 *metricsbp.Timer
	t1.ObserveDuration()
	var t2 metricsbp.Timer
	t2.ObserveDuration()
}
