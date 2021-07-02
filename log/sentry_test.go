package log

import (
	"context"
	"errors"
	"strings"
	"testing"

	sentry "github.com/getsentry/sentry-go"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type pair struct {
	key, value string
}

func TestExtractKeyValuePairs(t *testing.T) {
	for _, c := range []struct {
		label    string
		kv       []interface{}
		expected []pair
		dangling bool
	}{
		{
			label: "empty",
		},
		{
			label: "normal",
			kv: []interface{}{
				"key1", "value1",
				"key2", 2,
				"key3", 3.14,
			},
			expected: []pair{
				{
					key:   "key1",
					value: "value1",
				},
				{
					key:   "key2",
					value: "2",
				},
				{
					key:   "key3",
					value: "3.14",
				},
			},
		},
		{
			label: "ignore-zap-field",
			kv: []interface{}{
				"key1", "value1",
				"key2", "value2",
				zap.Field{},     // will be ignored
				zapcore.Field{}, // will be ignored
			},
			expected: []pair{
				{
					key:   "key1",
					value: "value1",
				},
				{
					key:   "key2",
					value: "value2",
				},
			},
		},
		{
			label: "dangling",
			kv: []interface{}{
				"key1", "value1",
				"dangling",
			},
			expected: []pair{
				{
					key:   "key1",
					value: "value1",
				},
			},
			dangling: true,
		},
		{
			label: "dangling-with-field",
			kv: []interface{}{
				"key1", "value1",
				zap.Field{},
				"dangling",
			},
			expected: []pair{
				{
					key:   "key1",
					value: "value1",
				},
			},
			dangling: true,
		},
	} {
		t.Run(
			c.label,
			func(t *testing.T) {
				var called int
				f := func(key, value string) {
					t.Helper()
					defer func() {
						called++
					}()
					if called >= len(c.expected) {
						t.Errorf("Extra call with (%q, %q)", key, value)
						return
					}
					if c.expected[called].key != key || c.expected[called].value != value {
						t.Errorf(
							"Expected %#v on %dth call, got (%q, %q)",
							c.expected[called],
							called,
							key, value,
						)
					}
				}
				dangling := extractKeyValuePairs(c.kv, f)
				if dangling != c.dangling {
					t.Errorf("Expected dangling to return %v, got %v", c.dangling, dangling)
				}
				if called < len(c.expected) {
					t.Errorf("Expected %d calls, got %v", len(c.expected), called)
				}
			},
		)
	}
}

func TestDropStackTraceFrame(t *testing.T) {
	t.Run("explicit error log", func(t *testing.T) {
		var (
			stackTraceBefore sentry.Stacktrace
			stackTraceAfter  sentry.Stacktrace
		)

		closer, err := InitSentry(SentryConfig{
			BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
				stackTraceBefore = *event.Exception[0].Stacktrace
				event = dropStackTraceFrame(event, hint)
				stackTraceAfter = *event.Exception[0].Stacktrace
				return event
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			err = closer.Close()
			if err != nil {
				t.Fatal(err)
			}
		})

		ErrorWithSentry(context.Background(), "test", errors.New("test"))

		if stackTraceBefore.Frames == nil {
			t.Error("expected stack trace but is nil")
		}
		if stackTraceAfter.Frames == nil {
			t.Error("expected stack trace but is nil")
		}
		if len(stackTraceAfter.Frames) == 0 {
			t.Error("expected stack trace to exists")
		}
		if len(stackTraceBefore.Frames)-1 != len(stackTraceAfter.Frames) {
			t.Error("expected top stack trace frame to be removed")
		}
		if !strings.HasSuffix(stackTraceBefore.Frames[0].AbsPath, "_test.go") {
			t.Errorf("expected top stack trace frame to point to test file, actual: %s", stackTraceBefore.Frames[0].AbsPath)
		}
	})

	t.Run("implicit through panic recover", func(t *testing.T) {
		closer, err := InitSentry(SentryConfig{
			BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
				return dropStackTraceFrame(event, hint)
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			err = closer.Close()
			if err != nil {
				t.Fatal(err)
			}
		})

		// does not generate stack trace frames, ensure the dropStackTraceFrame
		// method does not crash the recover process
		defer sentry.Recover()
		panic("exception")
	})
}
