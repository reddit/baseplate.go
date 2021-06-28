package mqsend

import (
	"context"
	"io"
)

// MessageQueueOpenMode is the mode used to open message queues.
const MessageQueueOpenMode = 0644

// MessageQueue represents a Posix Message Queue.
type MessageQueue interface {
	io.Closer

	// Send sends a message to the queue.
	//
	// If the context object does not have a deadline set,
	// or if the deadline had already passed when Send is called,
	// Send will be running in non-blocking mode and fail immediately when the
	// queue is full.
	Send(ctx context.Context, data []byte) error
}

// MessageQueueConfig is the config used in OpenMessageQueue call.
type MessageQueueConfig struct {
	// Name of the message queue, should not start with "/".
	Name string

	// The max number of messages in the queue.
	MaxQueueSize int64

	// The max size in bytes per message.
	MaxMessageSize int64
}

// OpenMessageQueue opens a named message queue.
//
// On Linux systems this returns the real thing.
// On non-linux systems this just returns a mocked version,
// see OpenMockMessageQueue.
func OpenMessageQueue(cfg MessageQueueConfig) (MessageQueue, error) {
	return openMessageQueue(cfg)
}
