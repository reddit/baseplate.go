package metricsbp_test

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/reddit/baseplate.go/metricsbp"
	"github.com/reddit/baseplate.go/set"
	"github.com/reddit/baseplate.go/tracing"
)

func runSpan(st *metricsbp.Statsd, spanErr error) (counter string, statusCounters set.String, histogram string, err error) {
	ctx, span := tracing.StartSpanFromThriftContext(context.Background(), "foo")
	time.Sleep(time.Millisecond)
	span.AddCounter("bar.count", 1.0)
	span.Stop(ctx, spanErr)

	var sb strings.Builder
	if _, err = st.Statsd.WriteTo(&sb); err != nil {
		return
	}
	stats := strings.Split(sb.String(), "\n")
	if len(stats) != 5 {
		err = fmt.Errorf("Expected 4 stats, got %d\n%v", len(stats)-1, stats)
		return
	}

	statusCounters = make(set.String)

	for _, stat := range stats {
		if strings.HasSuffix(stat, "|c") && strings.Contains(stat, "bar") {
			counter = stat
		} else if strings.HasSuffix(stat, "|c") {
			statusCounters.Add(stat)
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
			counter, statusCounters, histogram, err := runSpan(st, nil)
			if err != nil {
				t.Fatalf("Got error: %s", err)
			}

			expected := "bar.count:1.000000|c"
			if counter != expected {
				t.Errorf("Expected counter: %s\nGot: %s", expected, counter)
			}

			expectedSet := set.StringSliceToSet([]string{
				"server.foo.success:1.000000|c",
				"server.foo.total:1.000000|c",
			})
			if !statusCounters.Equals(expectedSet) {
				t.Errorf("Expected status counters: %v\nGot: %v", expectedSet, statusCounters)
			}

			if !histogramRegex.MatchString(histogram) {
				t.Errorf("Histogram %s did not match expected format", histogram)
			}
		},
	)

	t.Run(
		"fail",
		func(t *testing.T) {
			counter, statusCounters, histogram, err := runSpan(st, fmt.Errorf("test error"))
			if err != nil {
				t.Fatalf("Got error: %s", err)
			}

			expected := "bar.count:1.000000|c"
			if counter != expected {
				t.Errorf("Expected counter: %s\nGot: %s", expected, counter)
			}

			expectedSet := set.StringSliceToSet([]string{
				"server.foo.fail:1.000000|c",
				"server.foo.total:1.000000|c",
			})
			if !statusCounters.Equals(expectedSet) {
				t.Errorf("Expected status counters: %v\nGot: %v", expectedSet, statusCounters)
			}

			if !histogramRegex.MatchString(histogram) {
				t.Errorf("Histogram %s did not match expected format", histogram)
			}
		},
	)
}
