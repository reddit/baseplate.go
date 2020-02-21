package tracing

import (
	"context"
	"errors"
	"testing"
	"time"

	opentracing "github.com/opentracing/opentracing-go"

	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/mqsend"
)

const testTimeout = time.Millisecond * 100

func TestTracer(t *testing.T) {
	loggerFunc := func(t *testing.T) (logger log.Wrapper, called *bool) {
		called = new(bool)
		logger = func(msg string) {
			*called = true
			t.Logf("Logger called with msg: %q", msg)
		}
		return
	}

	t.Run(
		"too-large",
		func(t *testing.T) {
			recorder := mqsend.OpenMockMessageQueue(mqsend.MessageQueueConfig{
				MaxQueueSize:   1,
				MaxMessageSize: 1,
			})
			logger, called := loggerFunc(t)
			defer func() {
				CloseTracer()
				InitGlobalTracer(TracerConfig{})
			}()
			InitGlobalTracer(TracerConfig{
				SampleRate:               1,
				Logger:                   logger,
				TestOnlyMockMessageQueue: recorder,
			})
			// The above InitGlobalTracer might call the logger once for unable to get
			// ip, so clear the called state.
			*called = false

			span := AsSpan(opentracing.StartSpan("span"))
			err := span.Stop(context.Background(), nil)
			var e mqsend.MessageTooLargeError
			if !errors.As(err, &e) {
				t.Errorf("Expected MessageTooLargeError, got %v", err)
			}
			if !*called {
				t.Errorf("Expected logger called with span too big message, did not happen.")
			}
		},
	)

	recorder := mqsend.OpenMockMessageQueue(mqsend.MessageQueueConfig{
		MaxQueueSize:   1,
		MaxMessageSize: MaxSpanSize,
	})
	logger, called := loggerFunc(t)
	defer func() {
		CloseTracer()
		InitGlobalTracer(TracerConfig{})
	}()
	InitGlobalTracer(TracerConfig{
		SampleRate:               1,
		Logger:                   logger,
		TestOnlyMockMessageQueue: recorder,
	})
	// The above InitGlobalTracer might call the logger once for unable to get ip,
	// so clear the called state.
	*called = false

	t.Run(
		"first-message",
		func(t *testing.T) {
			span := AsSpan(opentracing.StartSpan("span"))
			err := span.Stop(context.Background(), nil)
			if err != nil {
				t.Errorf("End returned error: %v", err)
			}
			if *called {
				t.Errorf("Logger shouldn't be called with first span.")
			}

			// Clear called for the next subtest.
			*called = false
		},
	)

	t.Run(
		"second-message",
		func(t *testing.T) {
			span := AsSpan(opentracing.StartSpan("span"))
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			start := span.trace.start
			err := span.Stop(ctx, nil)
			duration := time.Since(start)
			if duration > DefaultMaxRecordTimeout*2 {
				t.Errorf(
					"Expected duration of around %v, got %v",
					DefaultMaxRecordTimeout,
					duration,
				)
			}
			if !errors.As(err, new(mqsend.TimedOutError)) {
				t.Errorf("Expected TimedOutError, got %v", err)
			}
			if !*called {
				t.Errorf("Expected logger called with time out message, did not happen.")
			}

			// Clear called for the next subtest.
			*called = false
		},
	)

	t.Run(
		"show-message",
		func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
			defer cancel()
			msg, err := recorder.Receive(ctx)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("Encoded span: %s", msg)
		},
	)
}
