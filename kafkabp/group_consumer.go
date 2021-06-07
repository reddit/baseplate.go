package kafkabp

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/Shopify/sarama"

	"github.com/reddit/baseplate.go/tracing"
)

type groupConsumer struct {
	consumer sarama.ConsumerGroup
	cfg      ConsumerConfig

	wg sync.WaitGroup

	consumeReturned int64
	closed          int64
}

// newGroupConsumer creates a new group Consumer.
func newGroupConsumer(cfg ConsumerConfig, sc *sarama.Config) (Consumer, error) {
	consumer, err := sarama.NewConsumerGroup(cfg.Brokers, cfg.GroupID, sc)
	if err != nil {
		return nil, err
	}
	return &groupConsumer{
		consumer: consumer,
		cfg:      cfg,
	}, nil
}

func (gc *groupConsumer) Consume(
	messagesFunc ConsumeMessageFunc,
	errorsFunc ConsumeErrorFunc,
) error {
	defer atomic.StoreInt64(&gc.consumeReturned, 1)
	gc.wg.Add(1)
	defer gc.wg.Done()

	gc.wg.Add(1)
	go func() {
		defer gc.wg.Done()
		for err := range gc.consumer.Errors() {
			errorsFunc(err)
		}
	}()

	handler := GroupConsumerHandler{
		Callback: messagesFunc,
		Topic:    gc.cfg.Topic,
	}

	// gc.consumer.Consume returns when either:
	// - rebalance happens
	// - Close was called
	for atomic.LoadInt64(&gc.closed) == 0 {
		if err := gc.consumer.Consume(
			context.Background(),
			[]string{gc.cfg.Topic},
			handler,
		); err != nil {
			errorsFunc(fmt.Errorf("sarama.ConsumerGroup.Consume returned error: %w", err))
		}
	}

	return nil
}

// Close closes the consumer.
func (gc *groupConsumer) Close() error {
	atomic.StoreInt64(&gc.closed, 1)

	// wait for the Consume function to return
	defer gc.wg.Wait()

	return gc.consumer.Close()
}

func (gc *groupConsumer) IsHealthy(_ context.Context) bool {
	return atomic.LoadInt64(&gc.consumeReturned) == 0
}

// GroupConsumerHandler implements sarama.ConsumerGroupHandler.
//
// It's exported so that users of this library can write mocks to test their
// ConsumeMessageFunc implementation.
type GroupConsumerHandler struct {
	Callback ConsumeMessageFunc
	Topic    string
}

// Setup is run at the beginning of a new session, before ConsumeClaim.
func (h GroupConsumerHandler) Setup(sarama.ConsumerGroupSession) error {
	// Do nothing.
	return nil
}

// Cleanup is run at the end of a session,
// once all ConsumeClaim goroutines have exited.
func (h GroupConsumerHandler) Cleanup(sarama.ConsumerGroupSession) error {
	// Do nothing.
	return nil
}

// ConsumeClaim starts a consumer loop of ConsumerGroupClaim's Messages() chan.
func (h GroupConsumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for m := range claim.Messages() {
		// Wrap in anonymous function for easier defer.
		func() {
			ctx := context.Background()
			var span *tracing.Span
			spanName := "group-consumer." + h.Topic
			ctx, span = tracing.StartTopLevelServerSpan(ctx, spanName)
			defer func() {
				span.FinishWithOptions(tracing.FinishOptions{
					Ctx: ctx,
				}.Convert())
			}()

			h.Callback(ctx, m)
			session.MarkMessage(
				m,
				"", // metadata
			)
		}()
	}
	return nil
}
