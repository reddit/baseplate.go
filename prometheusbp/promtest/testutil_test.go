package promtest

import (
	"fmt"
	"math"
	"testing"
)

func TestAlmostEqual(t *testing.T) {
	cases := []struct {
		a, b float64
		exp  bool
	}{
		{0, 0, true},
		{1, 1, true},
		{-1, -1, true},
		{1e9, 1e9, true},
		{1e-9, 1e-9, true},
		{0, 1, false},
		{-1, 0, false},
		{1e9 - 1, 1e9, false},
		{1e-9, 1e-10, false},
		{0, 1e-10, false},
		{0, -1e-10, false},
		{math.NaN(), math.NaN(), true},
		{math.Inf(-1), math.NaN(), false},
		{math.Inf(0), math.NaN(), false},
		{math.Inf(1), math.NaN(), false},
		{math.Inf(-1), math.Inf(-1), true},
		{math.Inf(-1), math.Inf(0), false},
		{math.Inf(-1), math.Inf(1), false},
		{math.Inf(0), math.Inf(-1), false},
		{math.Inf(0), math.Inf(0), true},
		{math.Inf(0), math.Inf(1), true},
		{math.Inf(1), math.Inf(-1), false},
		{math.Inf(1), math.Inf(0), true},
		{math.Inf(1), math.Inf(1), true},
		{math.Inf(1), 0, false},
		{math.Inf(1), 1e60, false},
		{math.Inf(1), 1e-60, false},
		{math.NaN(), 1e60, false},
		{math.NaN(), 1e-60, false},
		{10 / 2, 5, true},
		{10/2 + 1e-10, 5, true}, // just outside threshold
		{10/2 - 1e-10, 5, true},
		{-10/2 + 1e-10, -5, true},
		{-10/2 - 1e-10, -5, true},
		{10e-9/2 + 1e-19, 5e-9, true},
		{10e-9/2 - 1e-19, 5e-9, true},
		{-10e-9/2 + 1e-19, -5e-9, true},
		{-10e-9/2 - 1e-19, -5e-9, true},
		{10e9/2 + 1, 5e9, true},
		{10e9/2 - 1, 5e9, true},
		{-10e9/2 + 1, -5e9, true},
		{-10e9/2 - 1, -5e9, true},
		{10/2 + 5e-9, 5, false}, // just inside threshold
		{10/2 - 5e-9, 5, false},
		{-10/2 + 5e-9, -5, false},
		{-10/2 - 5e-9, -5, false},
		{10e-9/2 + 5e-17, 5e-9, false},
		{10e-9/2 - 5e-17, 5e-9, false},
		{-10e-9/2 + 5e-17, -5e-9, false},
		{-10e-9/2 - 5e-17, -5e-9, false},
		{10e9/2 + 1e1, 5e9, false},
		{10e9/2 - 1e1, 5e9, false},
		{-10e9/2 + 1e1, -5e9, false},
		{-10e9/2 - 1e1, -5e9, false},
	}
	for _, c := range cases {
		name := fmt.Sprintf("%f?=%f", c.a, c.b)
		t.Run(name, func(t *testing.T) {
			res := almostEqual(c.a, c.b)
			if res != c.exp {
				if res {
					t.Errorf("%f == %f but they should NOT be", c.a, c.b)
				} else {
					t.Errorf("%f != %f but they should be", c.a, c.b)
				}
			}
		})
	}
}
