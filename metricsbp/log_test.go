package metricsbp_test

import (
	"context"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/reddit/baseplate.go/metricsbp"
)

type wrapper struct {
	tb     testing.TB
	called atomic.Int64
}

func (w *wrapper) log(_ context.Context, msg string) {
	w.called.Add(1)
	if w.tb != nil {
		w.tb.Logf("log called with %q", msg)
	}
}

func TestLogWrapper(t *testing.T) {
	const path = "foo.bar"
	origin := &wrapper{
		tb: t,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	statsd := metricsbp.NewStatsd(ctx, metricsbp.Config{
		BufferInMemoryForTesting: true,
	})
	wrapped := metricsbp.LogWrapper(metricsbp.LogWrapperArgs{
		Counter: path,
		Statsd:  statsd,
		Wrapper: origin.log,
	})

	const expected = 2
	wrapped.Log(ctx, "called 1")
	wrapped.Log(ctx, "called 2")

	if called := origin.called.Load(); called != expected {
		t.Errorf(
			"Expect origin log.Wrapper to be called %d times, actual %d",
			expected,
			called,
		)
	}

	var sb strings.Builder
	if _, err := statsd.WriteTo(&sb); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(sb.String(), "\n")

	// Expected counter line:
	// "foo.bar:2.000000|c"
	re := regexp.MustCompile(
		`^` +
			path +
			`:` +
			`([0-9.]*)` +
			`\|c` +
			`$`,
	)
	var matched bool
	for _, line := range lines {
		group := re.FindStringSubmatch(strings.TrimSpace(line))
		if len(group) == 0 {
			continue
		}
		count, err := strconv.ParseFloat(group[1], 64)
		if err != nil {
			t.Errorf("Unable to parse line %q: %v", line, err)
			continue
		}
		matched = true
		if math.Abs(count-expected) > 1e-9 {
			t.Errorf("Expected counter %v, got %v", expected, count)
		}
	}
	if !matched {
		t.Errorf("No matched lines found in statsd output: %s", sb.String())
	}
}

func TestLogWrapperAllEmpty(t *testing.T) {
	// Just make sure that it does not panic. No real tests
	wrapped := metricsbp.LogWrapper(metricsbp.LogWrapperArgs{})
	wrapped.Log(context.Background(), "Hello, world!")
}
