package kafkabp

import "github.com/Shopify/sarama"

// ConsumerConfig is the configuration struct passed to NewConsumer.
type ConsumerConfig struct {
	// Required.
	Brokers []string
	Topic   string

	// Optional. Defaults to DefaultSaramaConfig.
	SaramaConfig *sarama.Config

	// Optional. If set, overrides the client ID set in SaramaConfig.
	ClientID string

	// Optional. Defaults to OffsetOldest.
	Offset KafkaOffset

	// Optional. Defaults to true. Set to false to prevent a server span from
	// being created for every Kafka message consumed.
	Tracing *bool
}
