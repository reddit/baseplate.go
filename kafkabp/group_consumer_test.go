package kafkabp

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/Shopify/sarama"
)

func TestGroupConsumer(t *testing.T) {
	// mock producer side by injecting a mocked consumer group
	mockProducer := newMockConsumerGroup()

	// test implemenation of a consumer
	testConsumer := newTestConsumer()

	// inject mocked consumer group
	gc := &groupConsumer{
		consumer: mockProducer,
	}

	// start group consumer in the background and collect all errors
	errors := make(chan error)
	go func() {
		errors <- gc.Consume(testConsumer.Consume, testConsumer.Error)
	}()

	// expect two messages
	testConsumer.expectedMessages = 2

	// producer will delivery messages to test consumer
	mockProducer.messages <- &sarama.ConsumerMessage{
		Value: []byte(`{"data":"a"}`),
	}
	mockProducer.messages <- &sarama.ConsumerMessage{
		Value: []byte(`{"data":"b"}`),
	}
	mockProducer.claimsAssigned <- true

	// wait for processing to finish but fail if it blocks too long
	select {
	case <-testConsumer.processed:
	case <-time.After(1 * time.Second):
		t.Fatal("consumer did not process all messages")
	}

	expected := []string{
		`{"data":"a"}`, `{"data":"b"}`,
	}
	actual := testConsumer.messages

	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected %v, actual: %v", expected, actual)
	}

	// close consumer and verify returned error message
	gc.Close()
	err := <-errors
	if err != nil {
		t.Error(err)
	}
}

type testConsumer struct {
	expectedMessages int
	processed        chan bool
	messages         []string
}

func newTestConsumer() *testConsumer {
	return &testConsumer{
		processed: make(chan bool),
	}
}

func (c *testConsumer) Consume(ctx context.Context, m *sarama.ConsumerMessage) {
	c.messages = append(c.messages, string(m.Value))
	if len(c.messages) == c.expectedMessages {
		c.processed <- true
	}
}

func (c *testConsumer) Error(err error) {}

func (c *testConsumer) Close() error {
	close(c.processed)
	return nil
}

func newMockConsumerGroup() *mockConsumerGroup {
	return &mockConsumerGroup{
		claimsAssigned: make(chan bool),
		closed:         make(chan bool),
		messages:       make(chan *sarama.ConsumerMessage, 3),
		errors:         make(chan error, 3),
	}
}

type mockConsumerGroup struct {
	messages       chan *sarama.ConsumerMessage
	claimsAssigned chan bool
	closed         chan bool
	errors         chan error
}

func (c *mockConsumerGroup) Consume(ctx context.Context, topics []string, handler sarama.ConsumerGroupHandler) error {
	select {
	case <-c.closed:
		return nil
	case <-c.claimsAssigned:
	}
	go func() {
		err := handler.ConsumeClaim(&mockConsumerSession{}, &mockConsumerClaim{
			messages: c.messages,
		})
		select {
		case <-c.closed:
			return
		default:
			c.errors <- err
		}
	}()
	return nil
}

func (c *mockConsumerGroup) Errors() <-chan error {
	return c.errors
}

func (c *mockConsumerGroup) Close() error {
	close(c.closed)
	close(c.errors)
	close(c.messages)
	return nil
}

type mockConsumerSession struct {
	claims       map[string][]int32
	memberID     string
	generationID int32
	ctx          context.Context
}

func (s *mockConsumerSession) Claims() map[string][]int32 { return s.claims }
func (s *mockConsumerSession) MemberID() string           { return s.memberID }
func (s *mockConsumerSession) GenerationID() int32        { return s.generationID }
func (s *mockConsumerSession) MarkOffset(topic string, partition int32, offset int64, metadata string) {
}
func (s *mockConsumerSession) Commit() {}
func (s *mockConsumerSession) ResetOffset(topic string, partition int32, offset int64, metadata string) {
}
func (s *mockConsumerSession) MarkMessage(msg *sarama.ConsumerMessage, metadata string) {}
func (s *mockConsumerSession) Context() context.Context                                 { return s.ctx }

type mockConsumerClaim struct {
	topic               string
	partition           int32
	initialOffset       int64
	highwaterMarkOffset int64
	messages            chan *sarama.ConsumerMessage
}

func (c *mockConsumerClaim) Topic() string                            { return c.topic }
func (c *mockConsumerClaim) Partition() int32                         { return c.partition }
func (c *mockConsumerClaim) InitialOffset() int64                     { return c.initialOffset }
func (c *mockConsumerClaim) HighWaterMarkOffset() int64               { return c.highwaterMarkOffset }
func (c *mockConsumerClaim) Messages() <-chan *sarama.ConsumerMessage { return c.messages }
