package events

import (
	"context"
	"time"

	"github.com/reddit/baseplate.go/mqsend"

	"github.com/apache/thrift/lib/go/thrift"
)

// Configuration values for the message queue.
const (
	// Max size in bytes for a single, serialized event.
	MaxEventSize = 102400

	// Max size of the events allowed in the message queue at one time.
	MaxQueueSize = 10000

	// Prefix added to the message queue name.
	QueueNamePrefix = "events-"

	// The default MaxPutTimeout to be used.
	DefaultMaxPutTimeout = time.Millisecond * 50

	// The default message queue name for v2 events.
	DefaultV2Name = "v2"
)

var serializerPool = thrift.NewTSerializerPoolSizeFactory(MaxEventSize, thrift.NewTJSONProtocolFactory())

// A Queue is an event queue.
type Queue struct {
	queue      mqsend.MessageQueue
	maxTimeout time.Duration
}

// The Config used to initialize an event queue.
type Config struct {
	// The name of the message queue, should not contain the "events-" prefix.
	//
	// For v2 events, the default name (when passed in Name is empty) is "v2".
	Name string

	// The max timeout applied to Put function.
	//
	// If the passed in context object already has an earlier deadline set,
	// that deadline will be respected instead.
	// But if the passed in context is already canceled,
	// then we ignore it and create a new background context with this timeout.
	//
	// If MaxPutTimeout <= 0, DefaultMaxPutTimeout will be used instead.
	MaxPutTimeout time.Duration
}

// V2 initializes a new v2 event queue with default configurations.
func V2() (*Queue, error) {
	return V2WithConfig(Config{})
}

// V2WithConfig initializes a new v2 event queue.
func V2WithConfig(cfg Config) (*Queue, error) {
	name := cfg.Name
	if name == "" {
		name = DefaultV2Name
	}
	queue, err := mqsend.OpenMessageQueue(mqsend.MessageQueueConfig{
		Name:           QueueNamePrefix + name,
		MaxQueueSize:   MaxQueueSize,
		MaxMessageSize: MaxEventSize,
	})
	if err != nil {
		return nil, err
	}
	return v2WithConfig(cfg, queue), nil
}

func v2WithConfig(cfg Config, queue mqsend.MessageQueue) *Queue {
	maxTimeout := cfg.MaxPutTimeout
	if maxTimeout <= 0 {
		maxTimeout = DefaultMaxPutTimeout
	}

	return &Queue{
		queue:      queue,
		maxTimeout: maxTimeout,
	}
}

// Close closes the event queue.
//
// After Close is called, all Put calls will return errors.
func (q *Queue) Close() error {
	return q.queue.Close()
}

// Put serializes and puts an event into the event queue.
func (q *Queue) Put(ctx context.Context, event thrift.TStruct) error {
	if ctx.Err() != nil {
		// The request context is already canceled,
		// use background to make sure we are still able to send out events.
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, q.maxTimeout)
	defer cancel()

	data, err := serializerPool.Write(ctx, event)
	if err != nil {
		return err
	}

	return q.queue.Send(ctx, data)
}
