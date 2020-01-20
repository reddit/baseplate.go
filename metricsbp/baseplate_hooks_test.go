package metricsbp_test

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/reddit/baseplate.go/metricsbp"
	"github.com/reddit/baseplate.go/tracing"
)

func runSpan(st metricsbp.Statsd, spanErr error) (counter string, histogram string, err error) {
	ctx := context.Background()
	span := tracing.StartSpanFromThriftContext(ctx, "foo")
	time.Sleep(time.Millisecond)
	span.End(ctx, spanErr)

	var sb strings.Builder
	if _, err = st.Statsd.WriteTo(&sb); err != nil {
		return
	}
	stats := strings.Split(sb.String(), "\n")
	if len(stats) != 3 {
		err = fmt.Errorf("Expected 2 stats, got %d", len(stats)-1)
		return
	}

	for _, stat := range stats {
		if strings.HasSuffix(stat, "|c") {
			counter = stat
		} else if strings.HasSuffix(stat, "|ms") {
			histogram = stat
		}
	}
	return
}

func TestOnServerSpanCreate(t *testing.T) {
	sampleRate := 1.0
	st := metricsbp.NewStatsd(
		context.Background(),
		metricsbp.StatsdConfig{
			DefaultSampleRate: sampleRate,
		},
	)
	defer st.StopReporting()

	hookPrefix := "service.testing"
	hook := metricsbp.BaseplateHook{
		Prefix:  hookPrefix,
		Metrics: st,
	}
	tracing.RegisterBaseplateHook(hook)
	defer tracing.ResetHooks()

	histogramRegex := regexp.MustCompile(`^service\.testing\.foo:\d\.\d+\|ms$`)

	t.Run(
		"success",
		func(t *testing.T) {
			counter, histogram, err := runSpan(st, nil)
			if err != nil {
				t.Fatalf("Got error: %s", err)
			}

			expectedCounter := "service.testing.foo.success:1.000000|c"
			if counter != expectedCounter {
				t.Errorf(
					"Expected counter: %s\nGot counter: %s",
					expectedCounter,
					counter,
				)
			}

			if !histogramRegex.MatchString(histogram) {
				t.Errorf(
					"Histogram %s did not match expected format",
					histogram,
				)
			}
		},
	)

	t.Run(
		"fail",
		func(t *testing.T) {
			counter, histogram, err := runSpan(st, fmt.Errorf("test error"))
			if err != nil {
				t.Fatalf("Got error: %s", err)
			}

			expectedCounter := "service.testing.foo.fail:1.000000|c"
			if counter != expectedCounter {
				t.Errorf(
					"Expected counter: %s\nGot counter: %s",
					expectedCounter,
					counter,
				)
			}

			if !histogramRegex.MatchString(histogram) {
				t.Errorf(
					"Histogram %s did not match expected format",
					histogram,
				)
			}
		},
	)
}
