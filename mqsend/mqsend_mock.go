package mqsend

import (
	"context"
	"time"
)

// MockMessageQueue is a mocked implementation of MessageQueue.
//
// It's implemented with channels.
type MockMessageQueue struct {
	msgs chan []byte

	maxSize int
}

// OpenMockMessageQueue creates a MockMessageQueue.
//
// The name from the cfg will be ignored.
func OpenMockMessageQueue(cfg MessageQueueConfig) *MockMessageQueue {
	return &MockMessageQueue{
		msgs:    make(chan []byte, cfg.MaxQueueSize),
		maxSize: int(cfg.MaxMessageSize),
	}
}

// Close closes the queue.
func (mmq *MockMessageQueue) Close() error {
	close(mmq.msgs)
	return nil
}

// Send sends a message to the queue.
func (mmq *MockMessageQueue) Send(ctx context.Context, data []byte) error {
	if len(data) > mmq.maxSize {
		return MessageTooLargeError{
			MessageSize: len(data),
			MaxSize:     mmq.maxSize,
		}
	}

	if deadline, ok := ctx.Deadline(); !ok || deadline.Before(time.Now()) {
		// Use non-block mode
		select {
		case mmq.msgs <- data:
			return nil
		default:
			return TimedOutError{
				Cause: context.DeadlineExceeded,
			}
		}
	}

	select {
	case mmq.msgs <- data:
		return nil
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			return TimedOutError{
				Cause: ctx.Err(),
			}
		}
		return ctx.Err()
	}
}

// Receive receives a message from the queue.
func (mmq *MockMessageQueue) Receive(ctx context.Context) ([]byte, error) {
	select {
	case msg := <-mmq.msgs:
		return msg, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
