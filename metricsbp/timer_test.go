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
	if v > m.expected*10 || v < m.expected/10 {
		m.t.Fail()
	}
}

func TestTimer(t *testing.T) {
	const sleep = time.Millisecond * 100
	h := mockHistogram{
		t:        t,
		expected: float64(sleep / time.Millisecond),
	}
	timer := metricsbp.NewTimer(&h)
	time.Sleep(sleep)
	timer.ObserveDuration()
	if !h.called {
		t.Errorf("histogram.Observe not called!")
	}
}

func TestTimerOverride(t *testing.T) {
	const duration = time.Second
	h := mockHistogram{
		t:        t,
		expected: float64(duration / time.Millisecond),
	}

	start, err := time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
	if err != nil {
		// Should not happen
		t.Fatal(err)
	}
	timer := metricsbp.NewTimer(&h)
	timer.OverrideStartTime(start)
	end := start.Add(duration)
	timer.ObserveWithEndTime(end)
	if !h.called {
		t.Error("histogram.Observe not called")
	}
}

func TestTimerZero(_ *testing.T) {
	// Just make sure the code doesn't panic here, no actual tests.

	var t1 *metricsbp.Timer
	t1.Start()
	t1.ObserveDuration()
	t1.OverrideStartTime(time.Now())
	t1.ObserveWithEndTime(time.Now())

	var t2 metricsbp.Timer
	t2.Start()
	t2.ObserveDuration()
	t2.OverrideStartTime(time.Now())
	t2.ObserveWithEndTime(time.Now())
}
