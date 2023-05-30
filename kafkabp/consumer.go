package kafkabp

import (
	"context"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Shopify/sarama"
	"github.com/prometheus/client_golang/prometheus"

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
//	consumer.Consume(
//	  consumeMessageFunc,
//	  func(err error) {
//	    log.Errorw(
//	      context.Background(),
//	      "kafka consumer error",
//	      "err", err,
//	      // additional key value pairs, for example topic info
//	    )
//	    // a prometheus counter
//	    consumerErrorCounter.Inc()
//	  },
//	)
type ConsumeErrorFunc func(err error)

// ConsumePartitionFunc is a function type for application to specify which
// partitions of the topic to consume data from.
type ConsumePartitionFunc func(partitionID int32) bool

// ConsumePartitionFuncProvider is a function type for application to
// provide a lambda function of ConsumePartitionFunc pinned for specific
// number of partitions. This allows creation of ConsumePartitionFunc once per
// reset. All PartitionConsumers when created decide if a partition is to be
// skipped or selected for consumption based on decision handed out by same
// instance implementation of ConsumePartitionFunc.
type ConsumePartitionFuncProvider func(numPartitions int) ConsumePartitionFunc

// ConsumeAllPartitionsFunc is a ConsumePartitionFunc that is to be used to
// specify all partitions to be consumed by the topic consumer.
//
// This function always returns true, causing all partitions to be consumed.
func ConsumeAllPartitionsFunc(partitionID int32) bool {
	return true
}

// ConsumeAllPartitionsFuncProvider is a ConsumePartitionFuncProvider that
// always selects all partitions.
func ConsumeAllPartitionsFuncProvider(numPartitions int) ConsumePartitionFunc {
	return ConsumeAllPartitionsFunc
}

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

	consumer                  atomic.Pointer[sarama.Consumer]
	partitions                atomic.Pointer[[]int32]
	partitionConsumers        atomic.Pointer[[]sarama.PartitionConsumer]
	partitionSelectorProvider ConsumePartitionFuncProvider

	closed          atomic.Int64
	consumeReturned atomic.Int64

	offset int64

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
	if cfg.ConsumePartitionFuncProvider == nil {
		cfg.ConsumePartitionFuncProvider = ConsumeAllPartitionsFuncProvider
	}

	kc := &consumer{
		cfg:                       cfg,
		sc:                        sc,
		offset:                    sc.Consumer.Offsets.Initial,
		partitionSelectorProvider: cfg.ConsumePartitionFuncProvider,
	}

	// Initialize Sarama consumer and set atomic values.
	if err := kc.reset(); err != nil {
		return nil, err
	}

	return kc, nil
}

func (kc *consumer) getConsumer() sarama.Consumer {
	if loaded := kc.consumer.Load(); loaded != nil {
		return *loaded
	}
	return nil
}

func (kc *consumer) getPartitions() []int32 {
	if loaded := kc.partitions.Load(); loaded != nil {
		return *loaded
	}
	return nil
}

func (kc *consumer) getPartitionConsumers() []sarama.PartitionConsumer {
	if loaded := kc.partitionConsumers.Load(); loaded != nil {
		return *loaded
	}
	return nil
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

	rebalance := func() (err error) {
		defer func() {
			rebalanceTotalCounter.Inc()
			if err != nil {
				rebalanceFailureCounter.Inc()
			}
		}()

		c, err := sarama.NewConsumer(kc.cfg.Brokers, kc.sc)
		if err != nil {
			return err
		}

		partitions, err := c.Partitions(kc.cfg.Topic)
		if err != nil {
			c.Close()
			return err
		}

		kc.consumer.Store(&c)
		kc.partitions.Store(&partitions)
		return nil
	}

	return rebalance()
}

// Close closes all partition consumers first, then the parent consumer.
func (kc *consumer) Close() error {
	// Return early if closing is already in progress
	if !kc.closed.CompareAndSwap(0, 1) {
		return nil
	}

	if partitionConsumers := kc.getPartitionConsumers(); partitionConsumers != nil {
		for _, pc := range partitionConsumers {
			// leaves room to drain pc's message and error channels
			pc.AsyncClose()
		}
	}
	// wait for the Consume function to return
	kc.wg.Wait()
	if c := kc.getConsumer(); c != nil {
		return c.Close()
	}
	return nil
}

// Consume consumes Kafka messages and errors from each partition's consumer.
// It is necessary to call Close() on the KafkaConsumer instance once all
// operations are done with the consumer instance.
func (kc *consumer) Consume(
	messagesFunc ConsumeMessageFunc,
	errorsFunc ConsumeErrorFunc,
) error {
	defer kc.consumeReturned.Store(1)
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
		numPartitions := len(partitions)
		partitionsSelector := kc.partitionSelectorProvider(numPartitions)
		if partitionsSelector == nil {
			return ErrNilConsumePartitionFunc
		}
		partitionConsumers := make([]sarama.PartitionConsumer, 0, numPartitions)
		partitionsToConsume := make([]bool, numPartitions)
		for _, partition := range partitions {
			partitionsToConsume[partition] = partitionsSelector(partition)
		}

		for _, p := range partitions {
			if !partitionsToConsume[p] {
				// partition p is to be skipped.
				continue
			}
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
						defer func(start time.Time) {
							consumerTimer.With(prometheus.Labels{
								topicLabel: kc.cfg.Topic,
							}).Observe(time.Since(start).Seconds())
							span.FinishWithOptions(tracing.FinishOptions{
								Ctx: ctx,
							}.Convert())
						}(time.Now())

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
		kc.partitionConsumers.Store(&partitionConsumers)

		wg.Wait()

		// Close or AsyncClose was called, so exit. The call to Close handles
		// cleaning up the consumer.
		if kc.closed.Load() != 0 {
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
	return kc.consumeReturned.Load() == 0
}
