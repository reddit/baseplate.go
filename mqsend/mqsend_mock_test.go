package mqsend_test

import (
	"context"
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
		MaxQueueSize:   1,
	})
	defer mq.Close()

	t.Run(
		"message-too-large",
		func(t *testing.T) {
			data := make([]byte, max+1)
			err := mq.Send(context.Background(), data)
			if err == nil {
				t.Error("Expected an error when message is larger than the max size, got nil")
			}
		},
	)

	t.Run(
		"send",
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
		"send-timeout",
		func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			err := mq.Send(ctx, []byte(msg))
			if err == nil {
				t.Error("Expected an timeout error when the queue is full, got nil")
			}
		},
	)

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
