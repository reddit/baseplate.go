package metricsbp

import (
	"time"

	"github.com/go-kit/kit/metrics"
)

const timerUnit = float64(time.Millisecond)

// Timer is a timer wraps a histogram.
//
// It's very similar to go-kit's Timer, with a few differences:
//
// 1. The reporting unit is millisecond and non-changeable.
//
// 2. It's nil-safe (zero values of *Timer or Timer will be safe to call, but
// they are no-ops)
type Timer struct {
	Histogram metrics.Histogram

	start time.Time
}

// NewTimer creates a new Timer and records its start time.
func NewTimer(h metrics.Histogram) *Timer {
	timer := &Timer{Histogram: h}
	timer.Start()
	return timer
}

// Start records the start time for the Timer.
func (t *Timer) Start() {
	t.start = time.Now()
}

// ObserveDuration reports the time elapsed via the wrapped histogram.
//
// If either t or *t is zero value, it will be no-op.
//
// The reporting unit is millisecond.
func (t *Timer) ObserveDuration() {
	if t == nil || t.Histogram == nil || t.start.IsZero() {
		return
	}
	d := float64(time.Since(t.start)) / timerUnit
	if d < 0 {
		d = 0
	}
	t.Histogram.Observe(d)
}
