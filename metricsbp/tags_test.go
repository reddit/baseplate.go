package metricsbp_test

import (
	"reflect"
	"testing"

	"github.com/reddit/baseplate.go/metricsbp"
)

func TestTags(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		tags     metricsbp.Tags
		expected []string
	}{
		{
			name:     "nil",
			tags:     nil,
			expected: nil,
		},
		{
			name:     "one",
			tags:     metricsbp.Tags{"key": "value"},
			expected: []string{"key", "value"},
		},
	}

	for _, _c := range cases {
		c := _c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			asStatsd := c.tags.AsStatsdTags()
			if len(asStatsd) != len(c.tags)*2 {
				t.Fatalf("wrong size: %#v", asStatsd)
			}
			if !reflect.DeepEqual(c.expected, asStatsd) {
				t.Fatalf("tags do not match, expected %#v, got %#v", c.expected, asStatsd)
			}
		})
	}
}
