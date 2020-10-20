package kafkabp

import (
	"net"

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
//     - broker1
//     - broker2
//   topic: sample-topic
//   sarama:
//     localAddr:
//       Network: tcp
//       String: 127.0.0.1:9092
//     clientID: myclient
//     offset: oldest
type ConsumerConfig struct {
	// Required.
	Brokers []string `yaml:"brokers"`
	Topic   string   `yaml:"topic"`

	// Required. Populated with Sarama's sane default values, and can be overridden.
	SaramaConfig *SaramaConsumerConfig `yaml:"sarama"`
}

// NewSaramaConfig instantiates a sarama.Config with sane defaults from
// sarama.NewConfig(), overwritten by values parsed from cfg.SaramaConfig.
func (cfg ConsumerConfig) NewSaramaConfig() (*sarama.Config, error) {
	c := sarama.NewConfig()

	if cfg.SaramaConfig == nil {
		return nil, ErrSaramaConfigEmpty
	}
	scc := cfg.SaramaConfig

	if scc.LocalAddr == nil {
		return nil, ErrLocalAddrEmpty
	}
	c.Net.LocalAddr = scc.LocalAddr

	if scc.ClientID != "" {
		c.ClientID = scc.ClientID
	}

	switch scc.Offset {
	case "oldest":
		c.Consumer.Offsets.Initial = sarama.OffsetOldest
	case "newest":
		c.Consumer.Offsets.Initial = sarama.OffsetNewest
	default:
		return nil, ErrOffsetInvalid
	}

	// Return any errors that occurred while consuming on the Errors channel.
	c.Consumer.Return.Errors = true

	return c, nil
}

// SaramaConsumerConfig specifies Sarama settings relevant to a Kafka consumer.
type SaramaConsumerConfig struct {
	// Required. The local address to use when establishing a connection.
	LocalAddr net.Addr `yaml:"localAddr"`

	// Optional. A user-provided string sent with every request to the brokers
	// for logging, debugging, and auditing purposes. Defaults to "sarama".
	ClientID string `yaml:"clientID"`

	// Optional. Defaults to "oldest". Valid values are "oldest" and "newest".
	Offset string `yaml:"offset"`
}
