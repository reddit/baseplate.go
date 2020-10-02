package kafkabp

import "github.com/Shopify/sarama"

// ConsumerConfig is the configuration struct passed to NewConsumer.
type ConsumerConfig struct {
	// Required.
	Brokers []string `yaml:"brokers"`
	Topic   string   `yaml:"topic"`

	// Optional. Defaults to DefaultSaramaConfig.
	SaramaConfig SaramaConsumerConfig `yaml:"sarama"`

	// Optional. Defaults to OffsetOldest. Valid values are specified as
	// constants in sarama_wrapper.go.
	Offset int64 `yaml:"kafkaOffset"`
}

// NewSaramaConfig instantiates sarama.Config with sane defaults from
// sarama.NewConfig(), overwritten by any values parsed from cfg.SaramaConfig.
func (cfg ConsumerConfig) NewSaramaConfig() (*sarama.Config, error) {
	c := sarama.NewConfig()
}

// SaramaConsumerConfig specifies Sarama settings relevant to a Kafka consumer.
// It uses
type SaramaConsumerConfig struct {
	// Optional. A user-provided string sent with every request to the brokers
	// for logging, debugging, and auditing purposes. Defaults to "sarama".
	ClientID string `yaml:"clientID"`
}
