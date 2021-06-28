package mqsend_test

import (
	"context"
	"errors"
	"syscall"
	"testing"
	"time"

	"github.com/reddit/baseplate.go/mqsend"
)

func TestMockMessageQueue(t *testing.T) {
	const msg = "hello, world!"
	const max = len(msg)
	const timeout = time.Millisecond

	mq := mqsend.OpenMockMessageQueue(mqsend.MessageQueueConfig{
		MaxMessageSize: int64(max),
		MaxQueueSize:   4,
	})
	defer mq.Close()

	sharedTest(t, mq, msg, max, timeout)

	t.Run(
		"receive",
		func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			data, err := mq.Receive(ctx)
			if err != nil {
				t.Fatalf("Receive returned error: %v", err)
			}
			if string(data) != msg {
				t.Errorf("Expected to receive data %q, got %q", msg, data)
			}
		},
	)

	t.Run(
		"send-again",
		func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			err := mq.Send(ctx, []byte(msg))
			if err != nil {
				t.Errorf("Send returned error: %v", err)
			}
		},
	)
}

func sharedTest(t *testing.T, mq mqsend.MessageQueue, msg string, max int, timeout time.Duration) {
	t.Run(
		"message-too-large",
		func(t *testing.T) {
			data := make([]byte, max+1)
			err := mq.Send(context.Background(), data)
			if !errors.As(err, new(mqsend.MessageTooLargeError)) {
				t.Errorf(
					"Expected MessageTooLargeError when message is larger than the max size, got %v",
					err,
				)
			}
		},
	)

	t.Run(
		"send-1-with-timeout",
		func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			err := mq.Send(ctx, []byte(msg))
			if err != nil {
				t.Errorf("Send returned error: %v", err)
			}
		},
	)

	t.Run(
		"send-2-neg-timeout",
		func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), -1)
			defer cancel()
			err := mq.Send(ctx, []byte(msg))
			if err != nil {
				t.Errorf("Send returned error: %v", err)
			}
		},
	)

	t.Run(
		"send-3-zero-timeout",
		func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 0)
			defer cancel()
			err := mq.Send(ctx, []byte(msg))
			if err != nil {
				t.Errorf("Send returned error: %v", err)
			}
		},
	)

	t.Run(
		"send-4-non-block",
		func(t *testing.T) {
			ctx := context.Background()
			err := mq.Send(ctx, []byte(msg))
			if err != nil {
				t.Errorf("Send returned error: %v", err)
			}
		},
	)

	t.Run(
		"send-timeout",
		func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			err := mq.Send(ctx, []byte(msg))
			if !errors.As(err, new(mqsend.TimedOutError)) {
				t.Errorf("Expected TimedOutError when the queue is full, got %v", err)
			}
			if !errors.Is(err, syscall.ETIMEDOUT) && !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf(
					"Expected either ETIMEDOUT or context.DeadlineExceeded when the queue is full, got %v",
					err,
				)
			}
		},
	)

	t.Run(
		"send-neg-timeout",
		func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), -1)
			defer cancel()
			err := mq.Send(ctx, []byte(msg))
			if !errors.As(err, new(mqsend.TimedOutError)) {
				t.Errorf("Expected TimedOutError when the queue is full, got %v", err)
			}
			if !errors.Is(err, syscall.ETIMEDOUT) && !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf(
					"Expected either ETIMEDOUT or context.DeadlineExceeded when the queue is full, got %v",
					err,
				)
			}
		},
	)

	t.Run(
		"send-zero-timeout",
		func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 0)
			defer cancel()
			err := mq.Send(ctx, []byte(msg))
			if !errors.As(err, new(mqsend.TimedOutError)) {
				t.Errorf("Expected TimedOutError when the queue is full, got %v", err)
			}
			if !errors.Is(err, syscall.ETIMEDOUT) && !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf(
					"Expected either ETIMEDOUT or context.DeadlineExceeded when the queue is full, got %v",
					err,
				)
			}
		},
	)

	t.Run(
		"send-non-block",
		func(t *testing.T) {
			ctx := context.Background()
			err := mq.Send(ctx, []byte(msg))
			if !errors.As(err, new(mqsend.TimedOutError)) {
				t.Errorf("Expected TimedOutError when the queue is full, got %v", err)
			}
			if !errors.Is(err, syscall.ETIMEDOUT) && !errors.Is(err, syscall.EAGAIN) && !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf(
					"Expected either ETIMEDOUT or context.DeadlineExceeded when the queue is full, got %v",
					err,
				)
			}
		},
	)
}
