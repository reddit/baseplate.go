package kafkabp

import (
	"context"
	"io"
	"sync"
	"sync/atomic"

	"github.com/Shopify/sarama"

	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/metricsbp"
	"github.com/reddit/baseplate.go/tracing"
)

// ConsumeMessageFunc is a function type for consuming consumer messages.
type ConsumeMessageFunc func(ctx context.Context, msg *sarama.ConsumerMessage) error

// ConsumeErrorFunc is a function type for consuming consumer errors.
type ConsumeErrorFunc func(err error)

// consumer is an instance of a Kafka consumer.
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

// Consumer defines the interface of a consumer struct.
type Consumer interface {
	io.Closer

	Consume(ConsumeMessageFunc, ConsumeErrorFunc) error

	// IsHealthy returns false after Consume returns.
	IsHealthy() bool
}

// NewConsumer creates a new Kafka consumer. Unlike a group consumer (which
// delivers every message exactly once by having one ClientID assigned to every
// consumer in the group), this consumer is used for consuming some
// configuration or data by all running consumer instances. This is why the
// ClientID provided to NewConsumer's ConsumerConfig must be unique.
func NewConsumer(cfg ConsumerConfig) (Consumer, error) {
	sc, err := cfg.NewSaramaConfig()
	if err != nil {
		return nil, err
	}

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
			log.Warnw("Error closing the consumer", "err", err)
		}
	}

	rebalance := func() error {
		c, err := sarama.NewConsumer(kc.cfg.Brokers, kc.sc)
		if err != nil {
			return err
		}

		partitions, err := c.Partitions(kc.cfg.Topic)
		if err != nil {
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
						var err error
						var span *tracing.Span
						spanName := "consumer." + kc.cfg.Topic
						ctx, span = tracing.StartTopLevelServerSpan(ctx, spanName)
						defer func() {
							span.FinishWithOptions(tracing.FinishOptions{
								Ctx: ctx,
								Err: err,
							}.Convert())
						}()

						err = messagesFunc(ctx, m)
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
func (kc *consumer) IsHealthy() bool {
	return atomic.LoadInt64(&kc.consumeReturned) == 0
}
