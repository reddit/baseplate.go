package metricsbp_test

import (
	"reflect"
	"testing"

	"github.com/reddit/baseplate.go/metricsbp"
)

func TestLabels(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		labels   metricsbp.Labels
		expected []string
	}{
		{
			name:     "nil",
			labels:   nil,
			expected: nil,
		},
		{
			name:     "one",
			labels:   metricsbp.Labels{"key": "value"},
			expected: []string{"key", "value"},
		},
	}

	for _, _c := range cases {
		c := _c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			asStatsd := c.labels.AsStatsdLabels()
			if len(asStatsd) != len(c.labels)*2 {
				t.Fatalf("wrong size: %#v", asStatsd)
			}
			if !reflect.DeepEqual(c.expected, asStatsd) {
				t.Fatalf("labels do not match, expected %#v, got %#v", c.expected, asStatsd)
			}
		})
	}
}
