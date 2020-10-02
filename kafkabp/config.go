package kafkabp

import "github.com/Shopify/sarama"

// ConsumerConfig is the configuration struct passed to NewConsumer.
type ConsumerConfig struct {
	// Required.
	Brokers []string `yaml:"brokers"`
	Topic   string   `yaml:"topic"`

	// Optional. Defaults to DefaultSaramaConfig.
	SaramaConfig *sarama.Config

	// Optional. If set, overrides the client ID set in SaramaConfig.
	ClientID string `yaml:"clientID"`

	// Optional. Defaults to OffsetOldest.
	Offset int64 `yaml:"kafkaOffset"`
}
