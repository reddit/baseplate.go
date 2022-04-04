package log_test

import (
	"context"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/go-kit/kit/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/reddit/baseplate.go/log"
)

func TestLogWrapperNilSafe(t *testing.T) {
	// Just make sure log.Wrapper.Log is nil-safe, no real tests
	var logger log.Wrapper
	logger.Log(context.Background(), "Hello, world!")
	logger.ToThriftLogger()("Hello, world!")
	log.WrapToThriftLogger(nil)("Hello, world!")
}

func TestLogWrapperUnmarshalText(t *testing.T) {
	getActualFuncName := func(w log.Wrapper) string {
		// This returns something like:
		// "github.com/reddit/baseplate.go/log.ZapWrapper.func1"
		return runtime.FuncForPC(reflect.ValueOf(w).Pointer()).Name()
	}

	for _, c := range []struct {
		text     string
		err      bool
		expected string
	}{
		{
			text: "fancy",
			err:  true,
		},
		{
			text:     "",
			expected: "log.ErrorWithSentryWrapper",
		},
		{
			text:     "nop",
			expected: "log.NopWrapper",
		},
		{
			text:     "std",
			expected: "log.StdWrapper",
		},
		{
			text:     "zap",
			expected: "log.ZapWrapper",
		},
		{
			// Unfortunately there's no way to check that the arg passed into
			// ZapWrapper is correct.
			text:     "zap:error",
			expected: "log.ZapWrapper",
		},
		{
			text: "zap:error:key",
			err:  true, // expect error because of dangling key.
		},
		{
			text: "zap:error:key=value:extra",
			err:  true, // expect error because of extra.
		},
		{
			text:     "zap:info:key1=value1,key2=value2 with space",
			expected: "log.ZapWrapper",
		},
		{
			text: "zaperror",
			err:  true,
		},
		{
			text:     "sentry",
			expected: "log.ErrorWithSentryWrapper",
		},
	} {
		t.Run(c.text, func(t *testing.T) {
			var w log.Wrapper
			err := w.UnmarshalText([]byte(c.text))
			if c.err {
				if err == nil {
					t.Errorf(
						"Expected UnmarshalText to return error, got nil. Result is %q",
						getActualFuncName(w),
					)
				}
			} else {
				if err != nil {
					t.Errorf("Expected UnmarshalText to return nil error, got %v", err)
				}
				name := getActualFuncName(w)
				if !strings.Contains(name, c.expected) {
					t.Errorf("Expected function name to contain %q, got %q", c.expected, name)
				}
			}
		})
	}
}

var (
	_ log.Counter = (prometheus.Counter)(nil)
	_ log.Counter = (metrics.Counter)(nil)
)
