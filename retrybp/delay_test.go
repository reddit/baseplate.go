package retrybp

import (
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"
	"time"
)

type randomBase int64

func (randomBase) Generate(r *rand.Rand, _ int) reflect.Value {
	var v int64
	if r.Float64() < 0.1 {
		// For 10% chance, generate a negative number.
		v = -r.Int63n(int64(math.MaxInt64))
	} else {
		v = r.Int63n(int64(math.MaxInt64))
	}
	return reflect.ValueOf(randomBase(v))
}

var _ quick.Generator = randomBase(0)

func TestActualMaxNQuick(t *testing.T) {
	f := func(base randomBase) bool {
		actualBase := int64(base)
		if actualBase <= 0 {
			actualBase = 1
		}
		n := actualMaxN(time.Duration(base))
		m := uint64(actualBase) << uint64(n)
		if int64(m) <= 0 {
			t.Errorf("%d << %d overflows", actualBase, n)
		}
		n1 := n + 1
		m = uint64(actualBase) << uint64(n1)
		if int64(m) > 0 {
			t.Errorf("%d << (%d+1) does not overflow", actualBase, n)
		}
		return !t.Failed()
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestActualMaxN(t *testing.T) {
	for _, c := range []struct {
		base time.Duration
		maxN int
	}{
		{
			base: 0,
			maxN: 62,
		},
		{
			base: 1,
			maxN: 62,
		},
		{
			base: -1,
			maxN: 62,
		},
		{
			base: time.Millisecond,
			maxN: 43,
		},
	} {
		t.Run(
			fmt.Sprintf("%v", c.base),
			func(t *testing.T) {
				n := actualMaxN(c.base)
				if n != c.maxN {
					t.Errorf("actualMaxN(%v) expected %d, got %d", c.base, c.maxN, n)
				}
			},
		)
	}
}

func TestCappedExponentialBackoff(t *testing.T) {
	for _, c := range []struct {
		label     string
		n         uint
		initial   time.Duration
		maxDelay  time.Duration
		maxN      int
		maxJitter time.Duration
		// The range of the expected result
		min, max time.Duration
	}{
		{
			label:   "first-try",
			n:       0,
			initial: time.Millisecond,
			min:     time.Millisecond,
			max:     time.Millisecond,
		},
		{
			label:   "second-try",
			n:       1,
			initial: time.Millisecond,
			min:     2 * time.Millisecond,
			max:     2 * time.Millisecond,
		},
		{
			label:   "auto-max-n",
			n:       9999,
			initial: time.Millisecond,
			max:     time.Duration(math.MaxInt64),
		},
		{
			label:   "max-n-too-high",
			n:       9999,
			initial: time.Millisecond,
			maxN:    9998,
			max:     time.Duration(math.MaxInt64),
		},
		{
			label:   "max-n",
			n:       9999,
			initial: time.Millisecond,
			maxN:    1,
			min:     2 * time.Millisecond,
			max:     2 * time.Millisecond,
		},
		{
			label:    "max-delay",
			n:        9999,
			initial:  time.Millisecond,
			maxDelay: time.Second,
			max:      time.Second,
		},
		{
			label:     "max-delay-with-jitter",
			n:         9999,
			initial:   time.Millisecond,
			maxDelay:  time.Second,
			maxJitter: time.Millisecond,
			min:       time.Second,
			max:       time.Second + time.Millisecond,
		},
	} {
		t.Run(
			c.label,
			func(t *testing.T) {
				delay := cappedExponentialBackoffFunc(CappedExponentialBackoffArgs{
					InitialDelay: c.initial,
					MaxDelay:     c.maxDelay,
					MaxN:         c.maxN,
					MaxJitter:    c.maxJitter,
				})(c.n, nil)
				if delay < c.min || delay > c.max {
					t.Errorf("Delay %v not in range [%v, %v]", delay, c.min, c.max)
				}
			},
		)
	}
}

func TestCappedExponentialBackoffQuick(t *testing.T) {
	const (
		max = time.Duration(math.MaxInt64)
		n   = 9999
	)
	delayFunc := cappedExponentialBackoffFunc(CappedExponentialBackoffArgs{
		MaxJitter: max,
	})
	f := func() bool {
		delay := delayFunc(n, nil)
		if delay > max || delay <= 0 {
			t.Errorf("Delay result overflew: %v", delay)
		}
		return !t.Failed()
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}
