package kafkabp

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Shopify/sarama"
	"github.com/Shopify/sarama/mocks"
	"github.com/stretchr/testify/assert"
)

func TestNewConsumer(t *testing.T) {
	var cfg ConsumerConfig

	// Sarama config with no client ID set should not create a new consumer and
	// throw ErrClientIDEmpty
	cfg.SaramaConfig = &sarama.Config{}
	c, err := NewConsumer(cfg)
	assert.Nil(t, c)
	assert.Equal(t, ErrClientIDEmpty, err)

	// Config with no Brokers should not create a new consumer and throw
	// ErrBrokersEmpty
	cfg.ClientID = "test-client"
	c, err = NewConsumer(cfg)
	assert.Nil(t, c)
	assert.Equal(t, ErrBrokersEmpty, err)

	// Config with no Topic should not create a new consumer and throw
	// ErrTopicEmpty
	cfg.Brokers = []string{"broker-1", "broker-2"}
	c, err = NewConsumer(cfg)
	assert.Nil(t, c)
	assert.Equal(t, ErrTopicEmpty, err)
}

func TestKafkaConsumer_Consume(t *testing.T) {
	kc := getTestMockConsumer(t)
	pc, pc1 := setupPartitionConsumers(t, kc)
	kMsg := getTestKafkaMessage("key1", "value1")
	kMsg1 := getTestKafkaMessage("key2", "value2")
	kErr := errors.New("test error")
	pc.YieldMessage(kMsg)
	pc.YieldError(kErr)
	pc1.YieldMessage(kMsg1)
	pc1.YieldError(kErr)

	var consumedMsgs []*sarama.ConsumerMessage
	var msgLock sync.Mutex
	var consumedErrs []error
	var errLock sync.Mutex

	var wg sync.WaitGroup
	wg.Add(4) // 2 kMsgs and 2 kerrs
	// Use a goroutine here since kc.Consume is a blocking call
	go func() {
		kc.Consume(
			func(_ context.Context, msg *sarama.ConsumerMessage) error {
				msgLock.Lock()
				defer msgLock.Unlock()
				consumedMsgs = append(consumedMsgs, msg)
				wg.Done()
				return nil
			},
			func(err error) {
				errLock.Lock()
				defer errLock.Unlock()
				consumedErrs = append(consumedErrs, err)
				wg.Done()
			},
		)
	}()

	wg.Wait() // wait for all kafka messages and errors to be consumed

	assert.Equal(t, 2, len(consumedMsgs))
	assert.Equal(t, 2, len(consumedErrs))
	assert.True(t, containsMsg(consumedMsgs, kMsg))
	assert.True(t, containsMsg(consumedMsgs, kMsg1))
}

// This tests that when Close() is called on a KafkaConsumer instance
// the messages and errors channel for every partition consumer is
// drained before the parent consumer is closed.
func TestKafkaConsumer_Close(t *testing.T) {
	kc := getTestMockConsumer(t)
	pc, pc1 := setupPartitionConsumers(t, kc)
	pc.ExpectMessagesDrainedOnClose()
	pc.ExpectErrorsDrainedOnClose()
	pc1.ExpectMessagesDrainedOnClose()
	pc1.ExpectErrorsDrainedOnClose()

	var consumedMsgs []*sarama.ConsumerMessage
	var msgLock sync.Mutex
	var consumedErrs []error
	var errLock sync.Mutex

	// since kc.Consume is a blocking operation
	go func() {
		kc.Consume(
			func(_ context.Context, msg *sarama.ConsumerMessage) error {
				msgLock.Lock()
				defer msgLock.Unlock()
				consumedMsgs = append(consumedMsgs, msg)
				return nil
			},
			func(err error) {
				errLock.Lock()
				defer errLock.Unlock()
				consumedErrs = append(consumedErrs, err)
			},
		)
	}()

	time.Sleep(time.Millisecond) // give time for partition consumers to initialize

	// send messages and errors to partition consumers
	for i := 1; i <= 5; i++ {
		kMsg := getTestKafkaMessage("key1", "value1")
		kMsg1 := getTestKafkaMessage("key2", "value2")
		kErr := errors.New("kafka error")
		kErr1 := errors.New("kafka error 2")
		pc.YieldMessage(kMsg)
		pc.YieldError(kErr)
		pc1.YieldMessage(kMsg1)
		pc1.YieldError(kErr1)
	}

	// close kafkaConsumer and assert all
	// messages and error channels are drained
	kc.Close()

	assert.Equal(t, 10, len(consumedMsgs))
	assert.Equal(t, 10, len(consumedErrs))
}

// Helper functions

func getTestMockConsumer(t *testing.T) *consumer {
	c := &consumer{
		topic:  "kafkabp-test",
		offset: OffsetNewest,
	}
	consumer, partitions := createMockConsumer(t, c.topic)
	c.consumer.Store(consumer)
	c.partitions.Store(partitions)
	return c
}

func createMockConsumer(t *testing.T, topic string) (mc *mocks.Consumer, partitions []int32) {
	partitions = []int32{1, 2}
	mc = mocks.NewConsumer(t, nil)
	metaData := make(map[string][]int32)
	metaData[topic] = partitions
	mc.SetTopicMetadata(metaData)
	return mc, partitions
}

func setupPartitionConsumers(t *testing.T, kc *consumer) (*mocks.PartitionConsumer, *mocks.PartitionConsumer) {
	t.Helper()

	mc, ok := kc.getConsumer().(*mocks.Consumer)
	if !ok {
		t.Fatalf("kc.consumer is not *mocks.Consumer. %#v", kc.consumer)
	}
	pc := mc.ExpectConsumePartition(kc.topic, kc.getPartitions()[0], kc.offset)
	pc1 := mc.ExpectConsumePartition(kc.topic, kc.getPartitions()[1], kc.offset)
	return pc, pc1
}

func getTestKafkaMessage(key, value string) *sarama.ConsumerMessage {
	return &sarama.ConsumerMessage{
		Key:   []byte([]byte(key)),
		Value: []byte([]byte(value)),
	}
}

func containsMsg(msgs []*sarama.ConsumerMessage, msg *sarama.ConsumerMessage) bool {
	for _, m := range msgs {
		if string(m.Key) == string(msg.Key) && string(m.Value) == string(msg.Value) {
			return true
		}
	}
	return false
}

// mocks newConsumer func in kafka.go
// makes it possible to mock sarama.Consumer instance
func newMockConsumer(mc sarama.Consumer) func([]string, *sarama.Config) (sarama.Consumer, error) {
	return func([]string, *sarama.Config) (sarama.Consumer, error) {
		return mc, nil
	}
}
