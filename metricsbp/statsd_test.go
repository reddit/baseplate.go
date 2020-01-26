package metricsbp_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/reddit/baseplate.go/metricsbp"
)

func TestGlobalStatsd(t *testing.T) {
	// Make sure global statsd is safe to use and won't cause panics, no real
	// tests here:
	metricsbp.M.RunSysStats()
	metricsbp.M.Counter("counter").Add(1)
	metricsbp.M.Histogram("hitogram").Observe(1)
	metricsbp.M.Timing("timing").Observe(1)
	metricsbp.M.Gauge("gauge").Set(1)
}

func TestNilStatsd(t *testing.T) {
	var st *metricsbp.Statsd
	// Make sure nil *Statsd is safe to use and won't cause panics, no real
	// tests here:
	st.RunSysStats()
	st.Counter("counter").Add(1)
	st.Histogram("hitogram").Observe(1)
	st.Timing("timing").Observe(1)
	st.Gauge("gauge").Set(1)
}

func TestNoFallback(t *testing.T) {
	var buf bytes.Buffer

	prefix := "counter"
	st := metricsbp.NewStatsd(
		context.Background(),
		metricsbp.StatsdConfig{
			Prefix:            prefix,
			DefaultSampleRate: 1,
		},
	)
	st.Counter("foo").Add(1)
	buf.Reset()
	st.Statsd.WriteTo(&buf)
	str := buf.String()
	if !strings.HasPrefix(str, prefix) {
		t.Errorf("Expected prefix %q, got %q", prefix, str)
	}

	prefix = "histogram"
	st = metricsbp.NewStatsd(
		context.Background(),
		metricsbp.StatsdConfig{
			Prefix:            prefix,
			DefaultSampleRate: 1,
		},
	)
	st.Histogram("foo").Observe(1)
	buf.Reset()
	st.Statsd.WriteTo(&buf)
	str = buf.String()
	if !strings.HasPrefix(str, prefix) {
		t.Errorf("Expected prefix %q, got %q", prefix, str)
	}

	prefix = "timing"
	st = metricsbp.NewStatsd(
		context.Background(),
		metricsbp.StatsdConfig{
			Prefix:            prefix,
			DefaultSampleRate: 1,
		},
	)
	st.Timing("foo").Observe(1)
	buf.Reset()
	st.Statsd.WriteTo(&buf)
	str = buf.String()
	if !strings.HasPrefix(str, prefix) {
		t.Errorf("Expected prefix %q, got %q", prefix, str)
	}

	prefix = "gauge"
	st = metricsbp.NewStatsd(
		context.Background(),
		metricsbp.StatsdConfig{
			Prefix:            prefix,
			DefaultSampleRate: 1,
		},
	)
	st.Gauge("foo").Set(1)
	buf.Reset()
	st.Statsd.WriteTo(&buf)
	str = buf.String()
	if !strings.HasPrefix(str, prefix) {
		t.Errorf("Expected prefix %q, got %q", prefix, str)
	}
}

func BenchmarkStatsd(b *testing.B) {
	const (
		label      = "label"
		sampleRate = 1
	)

	initialLabels := map[string]string{
		"source": "test",
	}

	labels := []string{
		"testtype",
		"benchmark",
	}

	st := metricsbp.NewStatsd(
		context.Background(),
		metricsbp.StatsdConfig{
			DefaultSampleRate: sampleRate,
			Labels:            initialLabels,
		},
	)

	b.Run(
		"pre-create",
		func(b *testing.B) {
			b.Run(
				"histogram",
				func(b *testing.B) {
					m := st.Histogram(label)
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						m.Observe(1)
					}
				},
			)

			b.Run(
				"timing",
				func(b *testing.B) {
					m := st.Timing(label)
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						m.Observe(1)
					}
				},
			)

			b.Run(
				"counter",
				func(b *testing.B) {
					m := st.Counter(label)
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						m.Add(1)
					}
				},
			)

			b.Run(
				"gauge",
				func(b *testing.B) {
					m := st.Gauge(label)
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
						st.Histogram(label).Observe(1)
					}
				},
			)

			b.Run(
				"timing",
				func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						st.Timing(label).Observe(1)
					}
				},
			)

			b.Run(
				"counter",
				func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						st.Counter(label).Add(1)
					}
				},
			)

			b.Run(
				"gauge",
				func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						st.Gauge(label).Set(1)
					}
				},
			)
		},
	)

	b.Run(
		"on-the-fly-with-labels",
		func(b *testing.B) {
			b.Run(
				"histogram",
				func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						st.Histogram(label).With(labels...).Observe(1)
					}
				},
			)

			b.Run(
				"timing",
				func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						st.Timing(label).With(labels...).Observe(1)
					}
				},
			)

			b.Run(
				"counter",
				func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						st.Counter(label).With(labels...).Add(1)
					}
				},
			)

			b.Run(
				"gauge",
				func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						st.Gauge(label).With(labels...).Set(1)
					}
				},
			)
		},
	)
}
