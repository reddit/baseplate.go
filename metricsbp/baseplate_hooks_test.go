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

func runSpan(st *metricsbp.Statsd, spanErr error) (counter string, successCounter string, histogram string, err error) {
	ctx, span := tracing.StartSpanFromHeaders(context.Background(), "foo", tracing.Headers{})
	time.Sleep(time.Millisecond)
	span.AddCounter("bar.count", 1.0)
	span.Stop(ctx, spanErr)

	var sb strings.Builder
	if _, err = st.Statsd.WriteTo(&sb); err != nil {
		return
	}
	stats := strings.Split(sb.String(), "\n")
	if len(stats) != 4 {
		err = fmt.Errorf("Expected 3 stats, got %d\n%v", len(stats)-1, stats)
		return
	}

	for _, stat := range stats {
		if strings.HasSuffix(stat, "|c") && strings.Contains(stat, "bar") {
			counter = stat
		} else if strings.HasSuffix(stat, "|c") {
			successCounter = stat
		} else if strings.HasSuffix(stat, "|ms") {
			histogram = stat
		}
	}
	return
}

func TestOnCreateServerSpan(t *testing.T) {
	st := metricsbp.NewStatsd(
		context.Background(),
		metricsbp.StatsdConfig{},
	)

	hook := metricsbp.CreateServerSpanHook{Metrics: st}
	tracing.RegisterCreateServerSpanHooks(hook)
	defer tracing.ResetHooks()

	histogramRegex, err := regexp.Compile(`^server\.foo:\d\.\d+\|ms$`)
	if err != nil {
		t.Fatal(err)
	}

	t.Run(
		"success",
		func(t *testing.T) {
			counter, statusCounter, histogram, err := runSpan(st, nil)
			if err != nil {
				t.Fatalf("Got error: %s", err)
			}

			expected := "bar.count:1.000000|c"
			if counter != expected {
				t.Errorf("Expected counter: %s\nGot: %s", expected, counter)
			}

			expected = "server.foo.success:1.000000|c"
			if statusCounter != expected {
				t.Errorf("Expected status counter: %s\nGot: %s", expected, statusCounter)
			}

			if !histogramRegex.MatchString(histogram) {
				t.Errorf("Histogram %s did not match expected format", histogram)
			}
		},
	)

	t.Run(
		"fail",
		func(t *testing.T) {
			counter, statusCounter, histogram, err := runSpan(st, fmt.Errorf("test error"))
			if err != nil {
				t.Fatalf("Got error: %s", err)
			}

			expected := "bar.count:1.000000|c"
			if counter != expected {
				t.Errorf("Expected counter: %s\nGot: %s", expected, counter)
			}

			expected = "server.foo.fail:1.000000|c"
			if statusCounter != expected {
				t.Errorf("Expected status counter: %s\nGot: %s", expected, statusCounter)
			}

			if !histogramRegex.MatchString(histogram) {
				t.Errorf("Histogram %s did not match expected format", histogram)
			}
		},
	)
}
