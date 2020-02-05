package metricsbp

import (
	"fmt"
	"math"
	"testing"
)

func compareFloats(t *testing.T, expected, actual float64) {
	const epsilon = 1e-9
	t.Helper()

	if math.Abs(expected-actual) < epsilon {
		return
	}
	t.Errorf("Expected %v, got %v", expected, actual)
}

func TestConvertSampleRate(t *testing.T) {
	cases := []struct {
		value, expected float64
	}{
		{
			value:    0,
			expected: 1,
		},
		{
			value:    1,
			expected: 1,
		},
		{
			value:    -1,
			expected: 0,
		},
		{
			value:    0.1,
			expected: 0.1,
		},
		{
			value:    0.01,
			expected: 0.01,
		},
	}

	for _, c := range cases {
		t.Run(
			fmt.Sprintf("%v", c.value),
			func(t *testing.T) {
				compareFloats(t, c.expected, convertSampleRate(c.value))
			},
		)
	}
}
