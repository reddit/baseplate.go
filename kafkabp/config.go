package kafkabp

import (
	"github.com/Shopify/sarama"
	"github.com/reddit/baseplate.go/log"
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

	// Required. ClientID is a user-provided string sent with every request to
	// the brokers for logging, debugging, and auditing purposes. The default
	// Consumer implementation in this library expects every Consumer to have a
	// unique ClientID.
	//
	// The Kubernetes pod ID is usually a good candidate for this unique ID.
	ClientID string `yaml:"clientID"`

	// Optional. Defaults to "oldest". Valid values are "oldest" and "newest".
	Offset string `yaml:"offset"`

	// Optional. If non-nil, will be used to log errors. At present, this only
	// pertains to logging errors closing the existing consumer when calling
	// consumer.reset().
	Logger log.Wrapper `yaml:"-"`
}

// NewSaramaConfig instantiates a sarama.Config with sane consumer defaults
// from sarama.NewConfig(), overwritten by values parsed from cfg.
func (cfg *ConsumerConfig) NewSaramaConfig() (*sarama.Config, error) {
	// Validate input parameters.
	if len(cfg.Brokers) == 0 {
		return nil, ErrBrokersEmpty
	}

	if cfg.Topic == "" {
		return nil, ErrTopicEmpty
	}

	if cfg.ClientID == "" {
		return nil, ErrClientIDEmpty
	}

	var offset int64
	switch cfg.Offset {
	case "":
		// OffsetOldest is the "true" default case (in that it will be reached if
		// an offset isn't specified).
		fallthrough
	case "oldest":
		offset = sarama.OffsetOldest
	case "newest":
		offset = sarama.OffsetNewest
	default:
		return nil, ErrOffsetInvalid
	}

	c := sarama.NewConfig()

	c.Consumer.Offsets.Initial = offset

	// Return any errors that occurred while consuming on the Errors channel.
	c.Consumer.Return.Errors = true

	return c, nil
}
