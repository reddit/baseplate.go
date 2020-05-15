package metricsbp

import (
	"bufio"
	"bytes"
	"context"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/go-kit/kit/metrics"
)

func TestSampledHistogram(t *testing.T) {
	const rate = 0.3

	statsd := NewStatsd(
		context.Background(),
		StatsdConfig{},
	)
	statsdSampled := NewStatsd(
		context.Background(),
		StatsdConfig{
			HistogramSampleRate: Float64Ptr(rate),
		},
	)
	statsdWithLabels := NewStatsd(
		context.Background(),
		StatsdConfig{
			Labels: map[string]string{
				"foo": "bar",
			},
		},
	)
	statsdWithLabelsSampled := NewStatsd(
		context.Background(),
		StatsdConfig{
			HistogramSampleRate: Float64Ptr(rate),
			Labels: map[string]string{
				"foo": "bar",
			},
		},
	)

	type histoFunc = func(statsd *Statsd) metrics.Histogram
	histoNoLabel := func(statsd *Statsd) metrics.Histogram {
		return statsd.Histogram("histo")
	}
	histoLabel := func(statsd *Statsd) metrics.Histogram {
		return statsd.Histogram("histo").With("key", "value")
	}
	histoOverrideRate := func(statsd *Statsd) metrics.Histogram {
		return statsd.HistogramWithRate("histo", rate)
	}

	cases := []struct {
		label    string
		sampled  bool
		statsd   *Statsd
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
		{
			label:    "override-sampled",
			sampled:  true,
			statsd:   statsd,
			newHisto: histoOverrideRate,
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
				_, err := c.statsd.statsd.WriteTo(&buf)
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
	const rate = 0.3

	// Line examples:
	// counter:20.000000|c
	// counter,foo=bar,key=value:3.000000|c|@0.100000
	lineParser, err := regexp.Compile(`^counter.*:(.*)\|c`)
	if err != nil {
		t.Fatal(err)
	}
	rateParser, err := regexp.Compile(`\|c\|@(.*)$`)
	if err != nil {
		t.Fatal(err)
	}

	statsd := NewStatsd(
		context.Background(),
		StatsdConfig{
			CounterSampleRate: Float64Ptr(1.5),
		},
	)
	statsdSampled := NewStatsd(
		context.Background(),
		StatsdConfig{
			CounterSampleRate: Float64Ptr(rate),
		},
	)
	statsdWithLabels := NewStatsd(
		context.Background(),
		StatsdConfig{
			Labels: map[string]string{
				"foo": "bar",
			},
		},
	)
	statsdWithLabelsSampled := NewStatsd(
		context.Background(),
		StatsdConfig{
			CounterSampleRate: Float64Ptr(rate),
			Labels: map[string]string{
				"foo": "bar",
			},
		},
	)

	type counterFunc = func(statsd *Statsd) metrics.Counter
	counterNoLabel := func(statsd *Statsd) metrics.Counter {
		return statsd.Counter("counter")
	}
	counterLabel := func(statsd *Statsd) metrics.Counter {
		return statsd.Counter("counter").With("key", "value")
	}
	counterOverrideRate := func(statsd *Statsd) metrics.Counter {
		return statsd.CounterWithRate("counter", rate)
	}

	cases := []struct {
		label      string
		sampled    bool
		statsd     *Statsd
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
		{
			label:      "override-sampled",
			sampled:    true,
			statsd:     statsd,
			newCounter: counterOverrideRate,
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
				_, err := c.statsd.statsd.WriteTo(&buf)
				if err != nil {
					t.Fatal(err)
				}
				line := strings.TrimSpace(buf.String())
				matches := lineParser.FindStringSubmatch(line)
				if len(matches) < 2 {
					t.Fatalf("Unexpected line: %q", line)
				}
				value, err := strconv.ParseFloat(matches[1], 64)
				if err != nil {
					t.Fatalf("Failed to parse value %v: %v", matches[1], err)
				}
				rateMatches := rateParser.FindStringSubmatch(line)
				if c.sampled {
					if value >= n {
						t.Errorf(
							"Expected sampled counter, got value %v (vs. %v), line: %q",
							value,
							n,
							line,
						)
					}
					if len(rateMatches) < 2 {
						t.Fatalf("Sample rate not found in line: %q", line)
					}
					value, err := strconv.ParseFloat(rateMatches[1], 64)
					if err != nil {
						t.Fatalf(
							"Failed to parse rate value %v: %v, line: %q",
							rateMatches[1],
							err,
							line,
						)
					}
					if math.Abs(value-rate) > epsilon {
						t.Errorf(
							"Expected sample rate %v, got %v, line: %q",
							rate,
							value,
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
					if len(rateMatches) < 2 {
						// No rate reported, which is fine for 100% samples
						return
					}
					value, err := strconv.ParseFloat(rateMatches[1], 64)
					if err != nil {
						t.Fatalf(
							"Failed to parse rate value %v: %v, line: %q",
							rateMatches[1],
							err,
							line,
						)
					}
					if math.Abs(value-1) > epsilon {
						t.Errorf(
							"Expected sample rate %v, got %v, line: %q",
							1,
							value,
							line,
						)
					}
				}
			},
		)
	}
}
