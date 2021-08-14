package tracing

import (
	"context"
	"errors"
	"testing"
	"testing/quick"
	"time"

	opentracing "github.com/opentracing/opentracing-go"

	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/mqsend"
)

const testTimeout = time.Millisecond * 100

func TestTracer(t *testing.T) {
	const timeout = time.Millisecond * 10
	const doubleTimeout = timeout * 2

	loggerFunc := func(t *testing.T) (logger log.Wrapper, called *bool) {
		called = new(bool)
		logger = func(_ context.Context, msg string) {
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
				InitGlobalTracer(Config{})
			}()
			InitGlobalTracer(Config{
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
		InitGlobalTracer(Config{})
	}()
	InitGlobalTracer(Config{
		SampleRate:               1,
		Logger:                   logger,
		MaxRecordTimeout:         timeout,
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
			if duration > doubleTimeout {
				t.Errorf(
					"Expected duration of around %v, got %v",
					timeout,
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

func TestHexID64Quick(t *testing.T) {
	const expectedLength = 16
	f := func() bool {
		s := hexID64()
		if len(s) != expectedLength {
			t.Errorf("Expected id length to be %d, got %q", expectedLength, s)
		}
		for i, char := range s {
			if !(char >= '0' && char <= '9' || char >= 'a' && char <= 'f') {
				t.Errorf("The %d-th char of %q is not hex", i, s)
			}
		}
		return !t.Failed()
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
	t.Log(hexID64())
}

func BenchmarkHexID64(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			hexID64()
		}
	})
}
