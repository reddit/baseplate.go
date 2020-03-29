package mqsend_test

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/reddit/baseplate.go/mqsend"
)

func TestLinuxMessageQueue(t *testing.T) {
	if runtime.GOOS != `linux` || !strings.HasSuffix(runtime.GOARCH, "64") {
		t.Logf(
			"This test can only be run on 64-bit Linux, skipping on %s/%s",
			runtime.GOOS,
			runtime.GOARCH,
		)
		return
	}

	const name = "test-mq"
	const msg = "hello, world!"
	const max = len(msg)
	const timeout = time.Millisecond

	mq, err := mqsend.OpenMessageQueue(mqsend.MessageQueueConfig{
		Name:           name,
		MaxMessageSize: int64(max),
		MaxQueueSize:   1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer mq.Close()

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
}
