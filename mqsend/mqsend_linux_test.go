// +build linux

package mqsend

import (
	"context"
	"errors"
	"fmt"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/reddit/baseplate.go/randbp"
)

func deleteMessageQueue(t *testing.T, name string) error {
	nameBytes, err := unix.BytePtrFromString(name)
	if err != nil {
		return err
	}
	// From MQ_UNLiNK(3) manpage:
	// int mq_unlink(const char *name);
	_, _, errno := unix.Syscall(
		unix.SYS_MQ_UNLINK,
		uintptr(unsafe.Pointer(nameBytes)), // name
		0,                                  // unused
		0,                                  // unused
	)
	if errno != 0 {
		return errno
	}
	return nil
}

func TestMessageQueueLinux(t *testing.T) {
	const msg = "hello, world!"
	const max = len(msg)
	const timeout = time.Millisecond

	name := fmt.Sprintf("test-mq-%d", randbp.R.Uint64())

	mq, err := openMessageQueueLinux(MessageQueueConfig{
		Name:           name,
		MaxMessageSize: int64(max),
		MaxQueueSize:   1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer mq.Close()

	// Delete the mq created in this test.
	defer func() {
		if err := deleteMessageQueue(t, name); err != nil {
			t.Errorf("Failed to delete message queue %q: %v", name, err)
		}
	}()

	t.Run(
		"message-too-large",
		func(t *testing.T) {
			data := make([]byte, max+1)
			err := mq.Send(context.Background(), data)
			if !errors.As(err, new(MessageTooLargeError)) {
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
			if !errors.As(err, new(TimedOutError)) {
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
