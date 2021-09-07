package metricsbp_test

import (
	"context"
	"errors"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/opentracing/opentracing-go"

	"github.com/reddit/baseplate.go/metricsbp"
	"github.com/reddit/baseplate.go/tracing"
)

func runSpan(tb testing.TB, st *metricsbp.Statsd, span *tracing.Span, spanErr error) (counter string, statusCounters []string, histograms []string) {
	time.Sleep(time.Millisecond)
	span.AddCounter("bar.count", 1)
	span.FinishWithOptions(tracing.FinishOptions{
		Err: spanErr,
	}.Convert())
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
			histograms = append(histograms, stat)
		} else if stat != "" {
			unaccounted = append(unaccounted, stat)
		}
	}
	if len(unaccounted) > 0 {
		tb.Errorf("unaccounted for stats: %#v", unaccounted)
	}
	sort.Strings(statusCounters)
	return
}

func foundHistogram(histograms []string, pattern *regexp.Regexp) bool {
	for _, histo := range histograms {
		if pattern.MatchString(histo) {
			return true
		}
	}
	return false
}

func TestOnCreateServerSpan(t *testing.T) {
	st := metricsbp.NewStatsd(
		context.Background(),
		metricsbp.Config{},
	)

	hook := metricsbp.CreateServerSpanHook{Metrics: st}
	tracing.RegisterCreateServerSpanHooks(hook)
	defer tracing.ResetHooks()

	serverGenerator := func() *tracing.Span {
		_, span := tracing.StartSpanFromHeaders(context.Background(), "foo", tracing.Headers{})
		return span
	}
	clientGenerator := func() *tracing.Span {
		ctx, _ := tracing.StartSpanFromHeaders(context.Background(), "server", tracing.Headers{})
		span, _ := opentracing.StartSpanFromContext(
			ctx,
			"service.foo",
			tracing.SpanTypeOption{
				Type: tracing.SpanTypeClient,
			},
		)
		return tracing.AsSpan(span)
	}
	localGenerator := func() *tracing.Span {
		ctx, _ := tracing.StartSpanFromHeaders(context.Background(), "server", tracing.Headers{})
		span, _ := opentracing.StartSpanFromContext(
			ctx,
			"foo",
			tracing.SpanTypeOption{
				Type: tracing.SpanTypeLocal,
			},
		)
		return tracing.AsSpan(span)
	}

	for _, c := range []struct {
		label            string
		spanGenerator    func() *tracing.Span
		err              error
		expectedCounters []string
		expectedRates    [][]string
		expectedHistoREs []string
	}{
		{
			label:         "server-success",
			spanGenerator: serverGenerator,
			err:           nil,
			expectedCounters: []string{
				"bar.count,endpoint=foo:1.000000|c",
			},
			expectedRates: [][]string{
				{
					"baseplate.server.rate,endpoint=foo,success=True:1.000000|c",
				},
				{
					"baseplate.server.rate,success=True,endpoint=foo:1.000000|c",
				},
			},
			expectedHistoREs: []string{
				// Example: "baseplate.server.latency,endpoint=foo:3600000.000000|ms"
				`^baseplate\.server\.latency,endpoint=foo:\d+\.\d+\|ms$`,
			},
		},
		{
			label:         "server-failure",
			spanGenerator: serverGenerator,
			err:           errors.New("error"),
			expectedCounters: []string{
				"bar.count,endpoint=foo:1.000000|c",
			},
			expectedRates: [][]string{
				{
					"baseplate.server.rate,endpoint=foo,success=False:1.000000|c",
				},
				{
					"baseplate.server.rate,success=False,endpoint=foo:1.000000|c",
				},
			},
			expectedHistoREs: []string{
				`^baseplate\.server\.latency,endpoint=foo:\d+\.\d+\|ms$`,
			},
		},
		{
			label:         "client-success",
			spanGenerator: clientGenerator,
			err:           nil,
			expectedCounters: []string{
				"bar.count,endpoint=foo,client=service:1.000000|c",
				"bar.count,client=service,endpoint=foo:1.000000|c",
			},
			expectedRates: [][]string{
				{
					"baseplate.client.rate,endpoint=foo,success=True,client=service:1.000000|c",
				},
				{
					"baseplate.client.rate,success=True,endpoint=foo,client=service:1.000000|c",
				},
				{
					"baseplate.client.rate,endpoint=foo,client=service,success=True:1.000000|c",
				},
				{
					"baseplate.client.rate,success=True,client=service,endpoint=foo:1.000000|c",
				},
				{
					"baseplate.client.rate,client=service,endpoint=foo,success=True:1.000000|c",
				},
				{
					"baseplate.client.rate,client=service,success=True,endpoint=foo:1.000000|c",
				},
			},
			expectedHistoREs: []string{
				`^baseplate\.client\.latency,(endpoint=foo,client=service)|(client=service,endpoint=foo):\d+\.\d+\|ms$`,
			},
		},
		{
			label:         "client-failure",
			spanGenerator: clientGenerator,
			err:           errors.New("error"),
			expectedCounters: []string{
				"bar.count,endpoint=foo,client=service:1.000000|c",
				"bar.count,client=service,endpoint=foo:1.000000|c",
			},
			expectedRates: [][]string{
				{
					"baseplate.client.rate,endpoint=foo,success=False,client=service:1.000000|c",
				},
				{
					"baseplate.client.rate,success=False,endpoint=foo,client=service:1.000000|c",
				},
				{
					"baseplate.client.rate,endpoint=foo,client=service,success=False:1.000000|c",
				},
				{
					"baseplate.client.rate,success=False,client=service,endpoint=foo:1.000000|c",
				},
				{
					"baseplate.client.rate,client=service,endpoint=foo,success=False:1.000000|c",
				},
				{
					"baseplate.client.rate,client=service,success=False,endpoint=foo:1.000000|c",
				},
			},
			expectedHistoREs: []string{
				`^baseplate\.client\.latency,(endpoint=foo,client=service)|(client=service,endpoint=foo):\d+\.\d+\|ms$`,
			},
		},
		{
			label:         "local-success",
			spanGenerator: localGenerator,
			err:           nil,
			expectedCounters: []string{
				"bar.count,endpoint=foo:1.000000|c",
			},
			expectedRates: [][]string{
				{
					"baseplate.local.rate,endpoint=foo,success=True:1.000000|c",
				},
				{
					"baseplate.local.rate,success=True,endpoint=foo:1.000000|c",
				},
			},
			expectedHistoREs: []string{
				`^baseplate\.local\.latency,endpoint=foo:\d+\.\d+\|ms$`,
			},
		},
		{
			label:         "local-failure",
			spanGenerator: localGenerator,
			err:           errors.New("error"),
			expectedCounters: []string{
				"bar.count,endpoint=foo:1.000000|c",
			},
			expectedRates: [][]string{
				{
					"baseplate.local.rate,endpoint=foo,success=False:1.000000|c",
				},
				{
					"baseplate.local.rate,success=False,endpoint=foo:1.000000|c",
				},
			},
			expectedHistoREs: []string{
				`^baseplate\.local\.latency,endpoint=foo:\d+\.\d+\|ms$`,
			},
		},
	} {
		t.Run(c.label, func(t *testing.T) {
			span := c.spanGenerator()
			counter, statusCounters, histograms := runSpan(t, st, span, c.err)

			var counterMatched bool
			for _, expected := range c.expectedCounters {
				if counter == expected {
					counterMatched = true
					break
				}
			}
			if !counterMatched {
				t.Errorf("Expected counter to be one of %#v, got: %s", c.expectedCounters, counter)
			}

			var ratesMatched bool
			for _, expected := range c.expectedRates {
				if reflect.DeepEqual(statusCounters, expected) {
					ratesMatched = true
					break
				}
			}
			if !ratesMatched {
				t.Errorf(
					"Expected rate counters to be one of %#v, got: %#v",
					c.expectedRates,
					statusCounters,
				)
			}

			for _, reStr := range c.expectedHistoREs {
				re, err := regexp.Compile(reStr)
				if err != nil {
					t.Errorf("Failed to compile regexp %s: %v", reStr, err)
				} else if !foundHistogram(histograms, re) {
					t.Errorf("Histograms %#v did not match expected regexp %s", histograms, reStr)
				}
			}
		})
	}
}

func TestWithStartAndFinishTimes(t *testing.T) {
	startTime := time.Unix(1, 0)
	stopTime := startTime.Add(time.Hour)

	st := metricsbp.NewStatsd(
		context.Background(),
		metricsbp.Config{},
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

	var histograms []string
	var sb strings.Builder
	if _, err := st.WriteTo(&sb); err != nil {
		return
	}
	stats := strings.Split(sb.String(), "\n")
	for _, stat := range stats {
		if strings.HasSuffix(stat, "|ms") {
			histograms = append(histograms, stat)
		}
	}

	// The order of emitted histograms are indeterministic
	expected1 := []string{
		"baseplate.server.latency,endpoint=foo:3600000.000000|ms",
	}
	expected2 := []string{
		"baseplate.server.latency,endpoint=foo:3600000.000000|ms",
	}
	if !reflect.DeepEqual(histograms, expected1) && !reflect.DeepEqual(histograms, expected2) {
		t.Errorf(
			"histograms mismatch, expected one of %#v & %#v, got %#v",
			expected1,
			expected2,
			histograms,
		)
	}
}
