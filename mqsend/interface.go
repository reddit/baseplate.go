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
	// Caller should always call Send with a context object with deadline set,
	// or Send might block forever when the queue is full.
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
