package kafkabp

import (
	"testing"

	"github.com/Shopify/sarama"
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

	// Correct config should create a new consumer
	// ErrTopicEmpty
	cfg.Topic = "test-topic"
	c, err = NewConsumer(cfg)
	assert.Nil(t, err)
	assert.NotNil(t, c)
}
