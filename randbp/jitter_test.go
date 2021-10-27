package randbp_test

import (
	"math"
	"testing"
	"testing/quick"

	"github.com/reddit/baseplate.go/randbp"
)

func TestJitterRatio(t *testing.T) {
	t.Run("quick", func(t *testing.T) {
		f := func() bool {
			jitter := randbp.R.Float64()
			min := 1 - jitter
			max := 1 + jitter
			ratio := randbp.JitterRatio(jitter)
			if ratio < max && ratio > min {
				return true
			}
			t.Errorf(
				"Expected JitterRatio(%v) to be in range (%v, %v), got %v",
				jitter,
				min,
				max,
				ratio,
			)
			return false
		}
		if err := quick.Check(f, nil); err != nil {
			t.Error(err)
		}
	})

	t.Run("<=0", func(t *testing.T) {
		const epsilon = 1e-9
		f := func() bool {
			jitter := -randbp.R.Float64()
			ratio := randbp.JitterRatio(jitter)
			if math.Abs(1-ratio) > epsilon {
				t.Errorf(
					"Expected JitterRatio(%v) to be 1, got %v",
					jitter,
					ratio,
				)
				return false
			}
			return true
		}
		if err := quick.Check(f, nil); err != nil {
			t.Error(err)
		}
	})

	t.Run(">=1", func(t *testing.T) {
		const (
			min = 0
			max = 2
		)
		f := func() bool {
			jitter := 1 + randbp.R.Float64()
			ratio := randbp.JitterRatio(jitter)
			if ratio < max && ratio > min {
				return true
			}
			t.Errorf(
				"Expected JitterRatio(%v) to be in range (%v, %v), got %v",
				jitter,
				min,
				max,
				ratio,
			)
			return false
		}
		if err := quick.Check(f, nil); err != nil {
			t.Error(err)
		}
	})
}
