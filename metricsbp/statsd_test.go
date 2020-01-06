package metricsbp_test

import (
	"context"
	"testing"

	"github.com/reddit/baseplate.go/metricsbp"
)

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
	defer st.StopReporting()

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
