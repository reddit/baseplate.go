package metricsbp_test

import (
	"bytes"
	"context"
	"io"
	"math"
	"strings"
	"testing"

	"github.com/reddit/baseplate.go/metricsbp"
)

func TestGlobalStatsd(t *testing.T) {
	// Make sure global statsd is safe to use and won't cause panics, no real
	// tests here:
	metricsbp.M.RunSysStats()
	metricsbp.M.Counter("counter").Add(1)
	metricsbp.M.CounterWithRate(metricsbp.RateArgs{}).Add(1)
	metricsbp.M.Histogram("hitogram").Observe(1)
	metricsbp.M.HistogramWithRate(metricsbp.RateArgs{}).Observe(1)
	metricsbp.M.Timing("timing").Observe(1)
	metricsbp.M.TimingWithRate(metricsbp.RateArgs{}).Observe(1)
	metricsbp.M.Gauge("gauge").Set(1)
	metricsbp.M.RuntimeGauge("gauge").Set(1)
	metricsbp.M.WriteTo(io.Discard)
}

func TestNilStatsd(t *testing.T) {
	var st *metricsbp.Statsd
	// Make sure nil *Statsd is safe to use and won't cause panics, no real
	// tests here:
	st.RunSysStats()
	st.Counter("counter").Add(1)
	st.CounterWithRate(metricsbp.RateArgs{}).Add(1)
	st.Histogram("hitogram").Observe(1)
	st.HistogramWithRate(metricsbp.RateArgs{}).Observe(1)
	st.Timing("timing").Observe(1)
	st.TimingWithRate(metricsbp.RateArgs{}).Observe(1)
	st.Gauge("gauge").Set(1)
	st.RuntimeGauge("gauge").Set(1)
	st.WriteTo(io.Discard)
}

func TestNoFallback(t *testing.T) {
	var buf bytes.Buffer

	prefix := "counter"
	st := metricsbp.NewStatsd(
		context.Background(),
		metricsbp.Config{
			Namespace:                prefix,
			BufferInMemoryForTesting: true,
		},
	)
	st.Counter("foo").Add(1)
	buf.Reset()
	st.WriteTo(&buf)
	str := buf.String()
	if !strings.HasPrefix(str, prefix) {
		t.Errorf("Expected prefix %q, got %q", prefix, str)
	}

	prefix = "histogram"
	st = metricsbp.NewStatsd(
		context.Background(),
		metricsbp.Config{
			Namespace:                prefix,
			BufferInMemoryForTesting: true,
		},
	)
	st.Histogram("foo").Observe(1)
	buf.Reset()
	st.WriteTo(&buf)
	str = buf.String()
	if !strings.HasPrefix(str, prefix) {
		t.Errorf("Expected prefix %q, got %q", prefix, str)
	}

	prefix = "timing"
	st = metricsbp.NewStatsd(
		context.Background(),
		metricsbp.Config{
			Namespace:                prefix,
			BufferInMemoryForTesting: true,
		},
	)
	st.Timing("foo").Observe(1)
	buf.Reset()
	st.WriteTo(&buf)
	str = buf.String()
	if !strings.HasPrefix(str, prefix) {
		t.Errorf("Expected prefix %q, got %q", prefix, str)
	}

	prefix = "gauge"
	st = metricsbp.NewStatsd(
		context.Background(),
		metricsbp.Config{
			Namespace:                prefix,
			BufferInMemoryForTesting: true,
		},
	)
	st.Gauge("foo").Set(1)
	buf.Reset()
	st.WriteTo(&buf)
	str = buf.String()
	if !strings.HasPrefix(str, prefix) {
		t.Errorf("Expected prefix %q, got %q", prefix, str)
	}
}

func BenchmarkStatsd(b *testing.B) {
	const (
		tag        = "tag"
		sampleRate = 1
	)

	initialTags := map[string]string{
		"source": "test",
	}

	tags := []string{
		"testtype",
		"benchmark",
	}

	st := metricsbp.NewStatsd(
		context.Background(),
		metricsbp.Config{
			Tags:                     initialTags,
			BufferInMemoryForTesting: true,
		},
	)

	b.Run(
		"pre-create",
		func(b *testing.B) {
			b.Run(
				"histogram",
				func(b *testing.B) {
					m := st.Histogram(tag)
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						m.Observe(1)
					}
				},
			)

			b.Run(
				"timing",
				func(b *testing.B) {
					m := st.Timing(tag)
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						m.Observe(1)
					}
				},
			)

			b.Run(
				"counter",
				func(b *testing.B) {
					m := st.Counter(tag)
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						m.Add(1)
					}
				},
			)

			b.Run(
				"gauge",
				func(b *testing.B) {
					m := st.Gauge(tag)
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						m.Set(1)
					}
				},
			)
		},
	)

	b.Run(
		"on-the-fly",
		func(b *testing.B) {
			b.Run(
				"histogram",
				func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						st.Histogram(tag).Observe(1)
					}
				},
			)

			b.Run(
				"timing",
				func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						st.Timing(tag).Observe(1)
					}
				},
			)

			b.Run(
				"counter",
				func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						st.Counter(tag).Add(1)
					}
				},
			)

			b.Run(
				"gauge",
				func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						st.Gauge(tag).Set(1)
					}
				},
			)
		},
	)

	b.Run(
		"on-the-fly-with-tags",
		func(b *testing.B) {
			b.Run(
				"histogram",
				func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						st.Histogram(tag).With(tags...).Observe(1)
					}
				},
			)

			b.Run(
				"timing",
				func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						st.Timing(tag).With(tags...).Observe(1)
					}
				},
			)

			b.Run(
				"counter",
				func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						st.Counter(tag).With(tags...).Add(1)
					}
				},
			)

			b.Run(
				"gauge",
				func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						st.Gauge(tag).With(tags...).Set(1)
					}
				},
			)
		},
	)
}

func TestRateArgsReportingRate(t *testing.T) {
	const epsilon = 1e-9
	for _, c := range []struct {
		label    string
		expected float64
		args     metricsbp.RateArgs
	}{
		{
			label:    "1-nil",
			expected: 1,
			args: metricsbp.RateArgs{
				Rate: 1,
			},
		},
		{
			label:    "0-nil",
			expected: 0,
			args: metricsbp.RateArgs{
				Rate: 0,
			},
		},
		{
			label:    "0.5-nil",
			expected: 0.5,
			args: metricsbp.RateArgs{
				Rate: 0.5,
			},
		},
		{
			label:    "0.5-0.5",
			expected: 0.25,
			args: metricsbp.RateArgs{
				Rate:             0.5,
				AlreadySampledAt: metricsbp.Float64Ptr(0.5),
			},
		},
		{
			label:    "0.5-1.5",
			expected: 0.5,
			args: metricsbp.RateArgs{
				Rate:             0.5,
				AlreadySampledAt: metricsbp.Float64Ptr(1.5),
			},
		},
		{
			label:    "negative",
			expected: 0,
			args: metricsbp.RateArgs{
				Rate:             0.5,
				AlreadySampledAt: metricsbp.Float64Ptr(-1),
			},
		},
	} {
		t.Run(c.label, func(t *testing.T) {
			rate := c.args.ReportingRate()
			if math.Abs(rate-c.expected) > epsilon {
				t.Errorf(
					"Expected ReportingRate for %#v to be %v, got %v",
					c.args,
					c.expected,
					rate,
				)
			}
		})
	}
}
