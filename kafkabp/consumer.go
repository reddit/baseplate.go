package kafkabp

import (
	"context"
	"io"
	"sync"
	"sync/atomic"

	"github.com/Shopify/sarama"

	"github.com/reddit/baseplate.go/metricsbp"
	"github.com/reddit/baseplate.go/tracing"
)

// ConsumeMessageFunc is a function type for consuming consumer messages.
//
// The implementation is expected to handle all consuming errors.
// For example, if there was anything wrong with handling the message and it
// needs to be retried, the ConsumeMessageFunc implementation should handle the
// retry (usually put the message into a retry topic).
type ConsumeMessageFunc func(ctx context.Context, msg *sarama.ConsumerMessage)

// ConsumeErrorFunc is a function type for consuming consumer errors.
//
// Note that these are usually system level consuming errors (e.g. read from
// broker failed, etc.), not individual message consuming errors.
//
// In most cases the implementation just needs to log the error and emit a
// counter, for example:
//
//     consumer.Consume(
//       consumeMessageFunc,
//       func(err error) {
//         log.ErrorWithSentry(
//           context.Background(),
//           "kafka consumer error",
//           err,
//           // additional key value pairs, for example topic info
//         )
//         metricsbp.M.Counter("kafka.consumer.errors").With(/* key value pairs */).Add(1)
//       },
//     )
type ConsumeErrorFunc func(err error)

// Consumer defines the interface of a consumer struct.
//
// It's also a superset of (implements) baseplate.HealthChecker.
type Consumer interface {
	io.Closer

	Consume(ConsumeMessageFunc, ConsumeErrorFunc) error

	// IsHealthy returns false after Consume returns.
	IsHealthy(ctx context.Context) bool
}

// consumer implements a Kafka consumer.
type consumer struct {
	cfg ConsumerConfig
	sc  *sarama.Config

	consumer           atomic.Value // sarama.Consumer
	partitions         atomic.Value // []int32
	partitionConsumers atomic.Value // []sarama.PartitionConsumer

	closed          int64
	consumeReturned int64
	offset          int64

	wg sync.WaitGroup
}

// NewConsumer creates a new Kafka consumer.
//
// It creates one of the two different implementations of Kafka consumer,
// depending on whether GroupID in config is empty:
//
// - If GroupID is non-empty, it creates a consumer that is part of a consumer
// group (sharing the same GroupID). The group will guarantee that every message
// is delivered to one of the consumers in the group exactly once. This is
// suitable for the traditional exactly-once message queue consumer use cases.
//
// - If GroupID is empty, it creates a consumer that has the whole view of the
// topic. This implementation of Kafka consumer is suitable for use cases like
// deliver config/data through Kafka to services.
func NewConsumer(cfg ConsumerConfig) (Consumer, error) {
	sc, err := cfg.NewSaramaConfig()
	if err != nil {
		return nil, err
	}

	switch {
	default:
		return newTopicConsumer(cfg, sc)
	case cfg.GroupID != "":
		return newGroupConsumer(cfg, sc)
	}
}

func newTopicConsumer(cfg ConsumerConfig, sc *sarama.Config) (Consumer, error) {
	kc := &consumer{
		cfg:    cfg,
		sc:     sc,
		offset: sc.Consumer.Offsets.Initial,
	}

	// Initialize Sarama consumer and set atomic values.
	if err := kc.reset(); err != nil {
		return nil, err
	}

	return kc, nil
}

func (kc *consumer) getConsumer() sarama.Consumer {
	c, _ := kc.consumer.Load().(sarama.Consumer)
	return c
}

func (kc *consumer) getPartitions() []int32 {
	p, _ := kc.partitions.Load().([]int32)
	return p
}

func (kc *consumer) getPartitionConsumers() []sarama.PartitionConsumer {
	pc, _ := kc.partitionConsumers.Load().([]sarama.PartitionConsumer)
	return pc
}

// reset recreates the consumer and assigns partitions.
func (kc *consumer) reset() error {
	if c := kc.getConsumer(); c != nil {
		if err := c.Close(); err != nil {
			kc.cfg.Logger.Log(
				context.Background(),
				"kafkabp.consumer.reset: Error closing the consumer: "+err.Error(),
			)
		}
	}

	rebalance := func() error {
		c, err := sarama.NewConsumer(kc.cfg.Brokers, kc.sc)
		if err != nil {
			return err
		}

		partitions, err := c.Partitions(kc.cfg.Topic)
		if err != nil {
			c.Close()
			return err
		}

		kc.consumer.Store(c)
		kc.partitions.Store(partitions)
		return nil
	}

	err := rebalance()
	if err != nil {
		metricsbp.M.Counter("kafka.consumer.rebalance.failure").Add(1)
		return err
	}

	metricsbp.M.Counter("kafka.consumer.rebalance.success").Add(1)
	return nil
}

// Close closes all partition consumers first, then the parent consumer.
func (kc *consumer) Close() error {
	// Return early if closing is already in progress
	if !atomic.CompareAndSwapInt64(&kc.closed, 0, 1) {
		return nil
	}

	partitionConsumers := kc.getPartitionConsumers()
	for _, pc := range partitionConsumers {
		// leaves room to drain pc's message and error channels
		pc.AsyncClose()
	}
	// wait for the Consume function to return
	kc.wg.Wait()
	return kc.getConsumer().Close()
}

// Consume consumes Kafka messages and errors from each partition's consumer.
// It is necessary to call Close() on the KafkaConsumer instance once all
// operations are done with the consumer instance.
func (kc *consumer) Consume(
	messagesFunc ConsumeMessageFunc,
	errorsFunc ConsumeErrorFunc,
) error {
	defer atomic.StoreInt64(&kc.consumeReturned, 1)
	kc.wg.Add(1)
	defer kc.wg.Done()

	// Sarama could close the channels (and cause the goroutines to finish) in
	// two cases, where we want different behavior:
	//   - in case of partition rebalance: restart goroutines
	//   - in case of call to Close/AsyncClose: exit
	var wg sync.WaitGroup
	for {
		// create a partition consumer for each partition
		consumer := kc.getConsumer()
		partitions := kc.getPartitions()
		partitionConsumers := make([]sarama.PartitionConsumer, 0, len(partitions))

		for _, p := range partitions {
			partitionConsumer, err := consumer.ConsumePartition(kc.cfg.Topic, p, kc.offset)
			if err != nil {
				return err
			}
			partitionConsumers = append(partitionConsumers, partitionConsumer) // for closing individual partitions when Close() is called

			// consume partition consumer messages
			wg.Add(1)
			go func(pc sarama.PartitionConsumer) {
				defer wg.Done()
				for m := range pc.Messages() {
					// Wrap in anonymous function for easier defer.
					func() {
						ctx := context.Background()
						var span *tracing.Span
						spanName := "consumer." + kc.cfg.Topic
						ctx, span = tracing.StartTopLevelServerSpan(ctx, spanName)
						defer func() {
							span.FinishWithOptions(tracing.FinishOptions{
								Ctx: ctx,
							}.Convert())
						}()

						messagesFunc(ctx, m)
					}()
				}
			}(partitionConsumer)

			// consume partition consumer errors
			wg.Add(1)
			go func(pc sarama.PartitionConsumer) {
				defer wg.Done()
				for err := range pc.Errors() {
					errorsFunc(err)
				}
			}(partitionConsumer)
		}
		kc.partitionConsumers.Store(partitionConsumers)

		wg.Wait()

		// Close or AsyncClose was called, so exit. The call to Close handles
		// cleaning up the consumer.
		if atomic.LoadInt64(&kc.closed) != 0 {
			return nil
		}

		// Close was not called, so we've gotten here because Sarama closed the
		// message channel due to a partition rebalance. Reset the consumer and
		// restart the goroutines.
		if err := kc.reset(); err != nil {
			return err
		}
	}
}

// IsHealthy returns true until Consume returns, then false thereafter.
func (kc *consumer) IsHealthy(_ context.Context) bool {
	return atomic.LoadInt64(&kc.consumeReturned) == 0
}
