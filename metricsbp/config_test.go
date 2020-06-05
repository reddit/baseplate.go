package metricsbp_test

import (
	"reflect"
	"testing"

	yaml "gopkg.in/yaml.v2"

	"github.com/reddit/baseplate.go/metricsbp"
)

func float64Ptr(v float64) *float64 {
	return &v
}

func TestConfigParsing(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		body     string
		expected metricsbp.Config
	}{
		{
			name: "defaults",
			body: `
namespace: foo
endpoint: bar:8080
`,
			expected: metricsbp.Config{
				Namespace: "foo",
				Endpoint:  "bar:8080",
			},
		},
		{
			name: "sample-rates",
			body: `
namespace: foo
endpoint: bar:8080
counterSampleRate: 0.1
histogramSampleRate: 0.01
`,
			expected: metricsbp.Config{
				Namespace:           "foo",
				Endpoint:            "bar:8080",
				CounterSampleRate:   float64Ptr(0.1),
				HistogramSampleRate: float64Ptr(0.01),
			},
		},
		{
			name: "tags",
			body: `
namespace: foo
endpoint: bar:8080
tags:
 fizz: buzz
 alpha: omega
`,
			expected: metricsbp.Config{
				Namespace: "foo",
				Endpoint:  "bar:8080",
				Tags: metricsbp.Tags{
					"fizz":  "buzz",
					"alpha": "omega",
				},
			},
		},
	}

	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				var cfg metricsbp.Config
				if err := yaml.Unmarshal([]byte(c.body), &cfg); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(cfg, c.expected) {
					t.Errorf("configs do not match, expected %#v, got %#v", c.expected, cfg)
				}
			},
		)
	}
}
