package kafkabp

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Shopify/sarama"
	"github.com/Shopify/sarama/mocks"

	baseplate "github.com/reddit/baseplate.go"
)

// Make sure that Consumer also implements baseplate.HealthChecker.
var (
	_ baseplate.HealthChecker = Consumer(nil)
)

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
			func(_ context.Context, msg *sarama.ConsumerMessage) {
				msgLock.Lock()
				defer msgLock.Unlock()
				consumedMsgs = append(consumedMsgs, msg)
				wg.Done()
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

	if len(consumedMsgs) != 2 {
		t.Errorf("expected len(consumedMsgs) == 2, got %d", len(consumedMsgs))
	}
	if len(consumedErrs) != 2 {
		t.Errorf("expected len(consumedErrs) == 2, got %d", len(consumedErrs))
	}
	if !containsMsg(consumedMsgs, kMsg) {
		t.Errorf("expected consumedMsgs to contain kMsg, got %v", consumedMsgs)
	}
	if !containsMsg(consumedMsgs, kMsg1) {
		t.Errorf("expected consumedMsgs to contain kMsg, got %v", consumedMsgs)
	}
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
			func(_ context.Context, msg *sarama.ConsumerMessage) {
				msgLock.Lock()
				defer msgLock.Unlock()
				consumedMsgs = append(consumedMsgs, msg)
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

	if len(consumedMsgs) != 10 {
		t.Errorf("expected len(consumedMsgs) == 10, got %d", len(consumedMsgs))
	}
	if len(consumedErrs) != 10 {
		t.Errorf("expected len(consumedErrs) == 10, got %d", len(consumedErrs))
	}
}

// Helper functions

func getTestMockConsumer(t *testing.T) *consumer {
	cfg := ConsumerConfig{
		Brokers:  []string{"127.0.0.1:9090", "127.0.0.2:9090"},
		Topic:    "kafkabp-test",
		ClientID: "test-mock-consumer",
	}

	sc, _ := cfg.NewSaramaConfig()
	c := &consumer{
		cfg:    cfg,
		sc:     sc,
		offset: sc.Consumer.Offsets.Initial,
	}
	consumer, partitions := createMockConsumer(t, cfg.Topic)
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
	pc := mc.ExpectConsumePartition(kc.cfg.Topic, kc.getPartitions()[0], kc.offset)
	pc1 := mc.ExpectConsumePartition(kc.cfg.Topic, kc.getPartitions()[1], kc.offset)
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
