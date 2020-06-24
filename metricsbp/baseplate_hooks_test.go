package metricsbp_test

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	opentracing "github.com/opentracing/opentracing-go"

	"github.com/reddit/baseplate.go/metricsbp"
	"github.com/reddit/baseplate.go/tracing"
)

func runSpan(tb testing.TB, st *metricsbp.Statsd, spanErr error) (counter string, statusCounters []string, histogram string) {
	ctx, span := tracing.StartSpanFromHeaders(context.Background(), "foo", tracing.Headers{})
	time.Sleep(time.Millisecond)
	span.AddCounter("bar.count", 1.0)
	span.Stop(ctx, spanErr)
	var unaccounted []string

	var sb strings.Builder
	if _, err := st.WriteTo(&sb); err != nil {
		tb.Fatal(err)
	}
	stats := strings.Split(sb.String(), "\n")

	for _, stat := range stats {
		if strings.HasSuffix(stat, "|c") && strings.Contains(stat, "bar") {
			counter = stat
		} else if strings.HasSuffix(stat, "|c") {
			statusCounters = append(statusCounters, stat)
		} else if strings.HasSuffix(stat, "|ms") {
			histogram = stat
		} else if stat != "" {
			unaccounted = append(unaccounted, stat)
		}
	}
	if len(unaccounted) > 0 {
		tb.Fatalf("unaccounted for stats: %+v", unaccounted)
	}
	sort.Strings(statusCounters)
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
			counter, statusCounters, histogram := runSpan(t, st, nil)

			expectedCounter := "bar.count:1.000000|c"
			if counter != expectedCounter {
				t.Errorf("Expected counter: %s\nGot: %s", expectedCounter, counter)
			}

			expected := []string{
				"server.foo.success:1.000000|c",
				"server.foo.total:1.000000|c",
			}
			if !reflect.DeepEqual(statusCounters, expected) {
				t.Errorf(
					"Expected status counters: %+v, got: %+v",
					expected,
					statusCounters,
				)
			}

			if !histogramRegex.MatchString(histogram) {
				t.Errorf("Histogram %s did not match expected format", histogram)
			}
		},
	)

	t.Run(
		"failure",
		func(t *testing.T) {
			counter, statusCounters, histogram := runSpan(t, st, fmt.Errorf("test error"))

			expectedCounter := "bar.count:1.000000|c"
			if counter != expectedCounter {
				t.Errorf("Expected counter: %s\nGot: %s", expectedCounter, counter)
			}

			expected := []string{
				"server.foo.failure:1.000000|c",
				"server.foo.total:1.000000|c",
			}
			if !reflect.DeepEqual(statusCounters, expected) {
				t.Errorf(
					"Expected status counters: %+v, got: %+v",
					expected,
					statusCounters,
				)
			}

			if !histogramRegex.MatchString(histogram) {
				t.Errorf("Histogram %s did not match expected format", histogram)
			}
		},
	)
}

func TestWithStartAndFinishTimes(t *testing.T) {
	startTime := time.Unix(1, 0)
	stopTime := startTime.Add(time.Hour)

	st := metricsbp.NewStatsd(
		context.Background(),
		metricsbp.StatsdConfig{},
	)

	hook := metricsbp.CreateServerSpanHook{Metrics: st}
	tracing.RegisterCreateServerSpanHooks(hook)
	defer tracing.ResetHooks()

	s, ctx := opentracing.StartSpanFromContext(
		context.Background(),
		"foo",
		tracing.SpanTypeOption{Type: tracing.SpanTypeServer},
		opentracing.StartTime(startTime),
	)
	span := tracing.AsSpan(s)
	opts := tracing.FinishOptions{Ctx: ctx}.Convert()
	opts.FinishTime = stopTime
	span.FinishWithOptions(opts)

	var histogram string
	var sb strings.Builder
	if _, err := st.WriteTo(&sb); err != nil {
		return
	}
	stats := strings.Split(sb.String(), "\n")
	for _, stat := range stats {
		if strings.HasSuffix(stat, "|ms") {
			histogram = stat
			break
		}
	}

	expected := "server.foo:3600000.000000|ms"
	if strings.Compare(histogram, expected) != 0 {
		t.Errorf("histogram mismatch, expected %q, got %q", expected, histogram)
	}

}
