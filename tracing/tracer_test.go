package tracing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/mqsend"
	"github.com/reddit/baseplate.go/runtimebp"
)

func TestTracer(t *testing.T) {
	loggerFunc := func(t *testing.T) (logger log.Wrapper, called *bool) {
		called = new(bool)
		logger = func(msg string) {
			*called = true
			t.Logf("Logger called with msg: %q", msg)
		}
		return
	}

	ip, err := runtimebp.GetFirstIPv4()
	if err != nil {
		t.Logf("Unable to get local ip address: %v", err)
	}
	tracer := Tracer{
		SampleRate: 1,
		Endpoint: ZipkinEndpointInfo{
			ServiceName: "test-service",
			IPv4:        ip,
		},
	}

	tracer.Recorder = mqsend.OpenMockMessageQueue(mqsend.MessageQueueConfig{
		MaxQueueSize:   1,
		MaxMessageSize: 1,
	})

	t.Run(
		"too-large",
		func(t *testing.T) {
			logger, called := loggerFunc(t)
			tracer.Logger = logger
			span := tracer.NewTrace("span")
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

	tracer.Recorder = mqsend.OpenMockMessageQueue(mqsend.MessageQueueConfig{
		MaxQueueSize:   1,
		MaxMessageSize: MaxSpanSize,
	})

	t.Run(
		"first-message",
		func(t *testing.T) {
			logger, called := loggerFunc(t)
			tracer.Logger = logger
			span := tracer.NewTrace("span")
			err := span.Stop(context.Background(), nil)
			if err != nil {
				t.Errorf("End returned error: %v", err)
			}
			if *called {
				t.Errorf("Logger shouldn't be called with first span.")
			}
		},
	)

	t.Run(
		"second-message",
		func(t *testing.T) {
			logger, called := loggerFunc(t)
			tracer.Logger = logger
			span := tracer.NewTrace("span")
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
			var e mqsend.TimedOutError
			if !errors.As(err, &e) {
				t.Errorf("Expected TimedOutError, got %v", err)
			}
			if !*called {
				t.Errorf("Expected logger called with time out message, did not happen.")
			}
		},
	)

	t.Run(
		"show-message",
		func(t *testing.T) {
			mmq := tracer.Recorder.(*mqsend.MockMessageQueue)
			ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
			defer cancel()
			msg, err := mmq.Receive(ctx)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("Encoded span: %s", msg)
		},
	)
}
