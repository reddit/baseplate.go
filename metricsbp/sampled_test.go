package metricsbp_test

import (
	"bufio"
	"bytes"
	"context"
	"math"
	"regexp"
	"strconv"
	"testing"

	"github.com/reddit/baseplate.go/metricsbp"

	"github.com/go-kit/kit/metrics"
)

func TestSampledHistogram(t *testing.T) {
	statsd := metricsbp.NewStatsd(
		context.Background(),
		metricsbp.StatsdConfig{},
	)
	statsdSampled := metricsbp.NewStatsd(
		context.Background(),
		metricsbp.StatsdConfig{
			HistogramSampleRate: 0.3,
		},
	)
	statsdWithLabels := metricsbp.NewStatsd(
		context.Background(),
		metricsbp.StatsdConfig{
			Labels: map[string]string{
				"foo": "bar",
			},
		},
	)
	statsdWithLabelsSampled := metricsbp.NewStatsd(
		context.Background(),
		metricsbp.StatsdConfig{
			HistogramSampleRate: 0.3,
			Labels: map[string]string{
				"foo": "bar",
			},
		},
	)

	type histoFunc = func(statsd *metricsbp.Statsd) metrics.Histogram
	histoNoLabel := func(statsd *metricsbp.Statsd) metrics.Histogram {
		return statsd.Histogram("histo")
	}
	histoLabel := func(statsd *metricsbp.Statsd) metrics.Histogram {
		return statsd.Histogram("histo").With("key", "value")
	}

	cases := []struct {
		label    string
		sampled  bool
		statsd   *metricsbp.Statsd
		newHisto histoFunc
	}{
		{
			label:    "not-sampled-no-labels",
			sampled:  false,
			statsd:   statsd,
			newHisto: histoNoLabel,
		},
		{
			label:    "not-sampled-no-labels-with",
			sampled:  false,
			statsd:   statsd,
			newHisto: histoLabel,
		},
		{
			label:    "sampled-no-labels",
			sampled:  true,
			statsd:   statsdSampled,
			newHisto: histoNoLabel,
		},
		{
			label:    "sampled-no-labels-with",
			sampled:  true,
			statsd:   statsdSampled,
			newHisto: histoLabel,
		},
		{
			label:    "not-sampled-labels",
			sampled:  false,
			statsd:   statsdWithLabels,
			newHisto: histoNoLabel,
		},
		{
			label:    "not-sampled-labels-with",
			sampled:  false,
			statsd:   statsdWithLabels,
			newHisto: histoLabel,
		},
		{
			label:    "sampled-labels",
			sampled:  true,
			statsd:   statsdWithLabelsSampled,
			newHisto: histoNoLabel,
		},
		{
			label:    "sampled-labels-with",
			sampled:  true,
			statsd:   statsdWithLabelsSampled,
			newHisto: histoLabel,
		},
	}

	for _, c := range cases {
		t.Run(
			c.label,
			func(t *testing.T) {
				const n = 20
				var buf bytes.Buffer
				histo := c.newHisto(c.statsd)
				for i := 0; i < n; i++ {
					histo.Observe(1)
				}
				_, err := c.statsd.Statsd.WriteTo(&buf)
				if err != nil {
					t.Fatal(err)
				}
				wire := buf.String()
				reader := bufio.NewScanner(&buf)
				var lines int
				for reader.Scan() {
					lines++
				}
				sampled := lines < n
				if sampled != c.sampled {
					t.Errorf(
						"Expected sampled to be %v, got %d of %d lines.\nwire:\n%s",
						c.sampled,
						lines,
						n,
						wire,
					)
				}
			},
		)
	}
}

func TestSampledCounter(t *testing.T) {
	// Line examples:
	// counter:20.000000|c
	// counter,foo=bar,key=value:3.000000|c|@0.100000
	lineParser, err := regexp.Compile(`^counter.*:(.*)\|c`)
	if err != nil {
		t.Fatal(err)
	}

	statsd := metricsbp.NewStatsd(
		context.Background(),
		metricsbp.StatsdConfig{},
	)
	statsdSampled := metricsbp.NewStatsd(
		context.Background(),
		metricsbp.StatsdConfig{
			CounterSampleRate: 0.3,
		},
	)
	statsdWithLabels := metricsbp.NewStatsd(
		context.Background(),
		metricsbp.StatsdConfig{
			Labels: map[string]string{
				"foo": "bar",
			},
		},
	)
	statsdWithLabelsSampled := metricsbp.NewStatsd(
		context.Background(),
		metricsbp.StatsdConfig{
			CounterSampleRate: 0.3,
			Labels: map[string]string{
				"foo": "bar",
			},
		},
	)

	type counterFunc = func(statsd *metricsbp.Statsd) metrics.Counter
	counterNoLabel := func(statsd *metricsbp.Statsd) metrics.Counter {
		return statsd.Counter("counter")
	}
	counterLabel := func(statsd *metricsbp.Statsd) metrics.Counter {
		return statsd.Counter("counter").With("key", "value")
	}

	cases := []struct {
		label      string
		sampled    bool
		statsd     *metricsbp.Statsd
		newCounter counterFunc
	}{
		{
			label:      "not-sampled-no-labels",
			sampled:    false,
			statsd:     statsd,
			newCounter: counterNoLabel,
		},
		{
			label:      "not-sampled-no-labels-with",
			sampled:    false,
			statsd:     statsd,
			newCounter: counterLabel,
		},
		{
			label:      "sampled-no-labels",
			sampled:    true,
			statsd:     statsdSampled,
			newCounter: counterNoLabel,
		},
		{
			label:      "sampled-no-labels-with",
			sampled:    true,
			statsd:     statsdSampled,
			newCounter: counterLabel,
		},
		{
			label:      "not-sampled-labels",
			sampled:    false,
			statsd:     statsdWithLabels,
			newCounter: counterNoLabel,
		},
		{
			label:      "not-sampled-labels-with",
			sampled:    false,
			statsd:     statsdWithLabels,
			newCounter: counterLabel,
		},
		{
			label:      "sampled-labels",
			sampled:    true,
			statsd:     statsdWithLabelsSampled,
			newCounter: counterNoLabel,
		},
		{
			label:      "sampled-labels-with",
			sampled:    true,
			statsd:     statsdWithLabelsSampled,
			newCounter: counterLabel,
		},
	}

	for _, c := range cases {
		t.Run(
			c.label,
			func(t *testing.T) {
				const n = 100
				const epsilon = 1e-9

				var buf bytes.Buffer
				counter := c.newCounter(c.statsd)
				for i := 0; i < n; i++ {
					counter.Add(1)
				}
				_, err := c.statsd.Statsd.WriteTo(&buf)
				if err != nil {
					t.Fatal(err)
				}
				line := buf.String()
				matches := lineParser.FindStringSubmatch(line)
				if len(matches) < 2 {
					t.Fatalf("Unexpected line: %q", line)
				}
				value, err := strconv.ParseFloat(matches[1], 64)
				if err != nil {
					t.Fatalf("Failed to parse value %v: %v", matches[1], err)
				}
				if c.sampled {
					if value >= n {
						t.Errorf(
							"Expected sampled counter, got value %v (vs. %v), line: %q",
							value,
							n,
							line,
						)
					}
				} else {
					if math.Abs(value-n) > epsilon {
						t.Errorf(
							"Expected not sampled counter, got value %v (vs. %v), line: %q",
							value,
							n,
							line,
						)
					}
				}
			},
		)
	}
}
