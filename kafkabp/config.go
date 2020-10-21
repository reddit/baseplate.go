package kafkabp

import (
	"github.com/Shopify/sarama"
)

// ConsumerConfig can be used to configure a kafkabp Consumer.
//
// Can be deserialized from YAML.
//
// Example:
//
// kafka:
//   brokers:
//     - 127.0.0.1:9090
//     - 127.0.0.2:9090
//   topic: sample-topic
//   clientID: myclient
//   offset: oldest
type ConsumerConfig struct {
	// Required. Brokers specifies a slice of broker addresses.
	Brokers []string `yaml:"brokers"`

	// Required. Topic is used to specify the topic to consume.
	Topic string `yaml:"topic"`

	// Optional. A user-provided string sent with every request to the brokers
	// for logging, debugging, and auditing purposes. Defaults to "sarama".
	ClientID string `yaml:"clientID"`

	// Optional. Defaults to "oldest". Valid values are "oldest" and "newest".
	Offset string `yaml:"offset"`
}

// NewSaramaConfig instantiates a sarama.Config with sane defaults from
// sarama.NewConfig(), overwritten by values parsed from cfg.Overrides.
func (cfg *ConsumerConfig) NewSaramaConfig() (*sarama.Config, error) {
	c := sarama.NewConfig()

	// Return any errors that occurred while consuming on the Errors channel.
	c.Consumer.Return.Errors = true

	if cfg.ClientID != "" {
		c.ClientID = cfg.ClientID
	}

	switch cfg.Offset {
	case "oldest":
		c.Consumer.Offsets.Initial = sarama.OffsetOldest
	case "newest":
		c.Consumer.Offsets.Initial = sarama.OffsetNewest
	case "":
		// This is the "true" default case (in that it will be reached if an offset isn't specified).
		c.Consumer.Offsets.Initial = sarama.OffsetOldest
	default:
		return nil, ErrOffsetInvalid
	}

	return c, nil
}
