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
	statsdWithTags := NewStatsd(
		context.Background(),
		StatsdConfig{
			Tags: map[string]string{
				"foo": "bar",
			},
		},
	)
	statsdWithTagsSampled := NewStatsd(
		context.Background(),
		StatsdConfig{
			HistogramSampleRate: Float64Ptr(rate),
			Tags: map[string]string{
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
		return statsd.HistogramWithRate(RateArgs{
			Name: "histo",
			Rate: rate,
		})
	}
	histoAlreadySampled := func(statsd *Statsd) metrics.Histogram {
		return statsd.HistogramWithRate(RateArgs{
			Name:             "histo",
			Rate:             1,
			AlreadySampledAt: Float64Ptr(rate),
		})
	}
	histoOverrideAlreadySampled := func(statsd *Statsd) metrics.Histogram {
		return statsd.HistogramWithRate(RateArgs{
			Name:             "histo",
			Rate:             rate,
			AlreadySampledAt: Float64Ptr(rate),
		})
	}

	cases := []struct {
		tag      string
		sampled  bool
		statsd   *Statsd
		newHisto histoFunc
	}{
		{
			tag:      "not-sampled-no-tags",
			sampled:  false,
			statsd:   statsd,
			newHisto: histoNoLabel,
		},
		{
			tag:      "not-sampled-no-tags-with",
			sampled:  false,
			statsd:   statsd,
			newHisto: histoLabel,
		},
		{
			tag:      "sampled-no-tags",
			sampled:  true,
			statsd:   statsdSampled,
			newHisto: histoNoLabel,
		},
		{
			tag:      "sampled-no-tags-with",
			sampled:  true,
			statsd:   statsdSampled,
			newHisto: histoLabel,
		},
		{
			tag:      "not-sampled-tags",
			sampled:  false,
			statsd:   statsdWithTags,
			newHisto: histoNoLabel,
		},
		{
			tag:      "not-sampled-tags-with",
			sampled:  false,
			statsd:   statsdWithTags,
			newHisto: histoLabel,
		},
		{
			tag:      "sampled-tags",
			sampled:  true,
			statsd:   statsdWithTagsSampled,
			newHisto: histoNoLabel,
		},
		{
			tag:      "sampled-tags-with",
			sampled:  true,
			statsd:   statsdWithTagsSampled,
			newHisto: histoLabel,
		},
		{
			tag:      "override-sampled",
			sampled:  true,
			statsd:   statsd,
			newHisto: histoOverrideRate,
		},
		{
			tag:      "already-sampled",
			sampled:  false,
			statsd:   statsd,
			newHisto: histoAlreadySampled,
		},
		{
			tag:      "override-already-sampled",
			sampled:  true,
			statsd:   statsd,
			newHisto: histoOverrideAlreadySampled,
		},
	}

	for _, c := range cases {
		t.Run(
			c.tag,
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
	const (
		rate = 0.3

		alreadySampled     = 0.5
		alreadySampledRate = rate * alreadySampled
	)

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
	statsdWithTags := NewStatsd(
		context.Background(),
		StatsdConfig{
			Tags: map[string]string{
				"foo": "bar",
			},
		},
	)
	statsdWithTagsSampled := NewStatsd(
		context.Background(),
		StatsdConfig{
			CounterSampleRate: Float64Ptr(rate),
			Tags: map[string]string{
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
		return statsd.CounterWithRate(RateArgs{
			Name: "counter",
			Rate: rate,
		})
	}
	counterAlreadySampled := func(statsd *Statsd) metrics.Counter {
		return statsd.CounterWithRate(RateArgs{
			Name:             "counter",
			Rate:             rate,
			AlreadySampledAt: Float64Ptr(alreadySampled),
		})
	}

	cases := []struct {
		tag          string
		sampled      bool
		expectedRate float64
		statsd       *Statsd
		newCounter   counterFunc
	}{
		{
			tag:          "not-sampled-no-tags",
			sampled:      false,
			expectedRate: 1,
			statsd:       statsd,
			newCounter:   counterNoLabel,
		},
		{
			tag:          "not-sampled-no-tags-with",
			sampled:      false,
			expectedRate: 1,
			statsd:       statsd,
			newCounter:   counterLabel,
		},
		{
			tag:          "sampled-no-tags",
			sampled:      true,
			expectedRate: rate,
			statsd:       statsdSampled,
			newCounter:   counterNoLabel,
		},
		{
			tag:          "sampled-no-tags-with",
			sampled:      true,
			expectedRate: rate,
			statsd:       statsdSampled,
			newCounter:   counterLabel,
		},
		{
			tag:          "not-sampled-tags",
			sampled:      false,
			expectedRate: 1,
			statsd:       statsdWithTags,
			newCounter:   counterNoLabel,
		},
		{
			tag:          "not-sampled-tags-with",
			sampled:      false,
			expectedRate: 1,
			statsd:       statsdWithTags,
			newCounter:   counterLabel,
		},
		{
			tag:          "sampled-tags",
			sampled:      true,
			expectedRate: rate,
			statsd:       statsdWithTagsSampled,
			newCounter:   counterNoLabel,
		},
		{
			tag:          "sampled-tags-with",
			sampled:      true,
			expectedRate: rate,
			statsd:       statsdWithTagsSampled,
			newCounter:   counterLabel,
		},
		{
			tag:          "override-sampled",
			sampled:      true,
			expectedRate: rate,
			statsd:       statsd,
			newCounter:   counterOverrideRate,
		},
		{
			tag:          "already-sampled",
			sampled:      true,
			expectedRate: alreadySampledRate,
			statsd:       statsd,
			newCounter:   counterAlreadySampled,
		},
	}

	for _, c := range cases {
		t.Run(
			c.tag,
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
					if math.Abs(value-c.expectedRate) > epsilon {
						t.Errorf(
							"Expected sample rate %v, got %v, line: %q",
							c.expectedRate,
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
