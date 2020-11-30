package kafkabp

import (
	"errors"
	"fmt"

	"github.com/Shopify/sarama"

	"github.com/reddit/baseplate.go/log"
)

// Allowed Offset values
const (
	OffsetOldest = "oldest"
	OffsetNewest = "newest"
)

var (
	// ErrBrokersEmpty is thrown when the slice of brokers is empty.
	ErrBrokersEmpty = errors.New("kafkabp: Brokers are empty")

	// ErrTopicEmpty is thrown when the topic is empty.
	ErrTopicEmpty = errors.New("kafkabp: Topic is empty")

	// ErrClientIDEmpty is thrown when the client ID is empty.
	ErrClientIDEmpty = errors.New("kafkabp: ClientID is empty")

	// ErrOffsetInvalid is thrown when an invalid offset is specified.
	ErrOffsetInvalid = errors.New("kafkabp: Offset is invalid")
)

// ConsumerConfig can be used to configure a kafkabp Consumer.
//
// Can be deserialized from YAML.
//
// Example:
//
//     kafka:
//       brokers:
//         - 127.0.0.1:9090
//         - 127.0.0.2:9090
//       topic: sample-topic
//       clientID: myclient
//       version: 2.4.0
//       offset: oldest
type ConsumerConfig struct {
	// Required. Brokers specifies a slice of broker addresses.
	Brokers []string `yaml:"brokers"`

	// Required. Topic is used to specify the topic to consume.
	Topic string `yaml:"topic"`

	// Required. ClientID is used by Kafka broker to track clients' consuming
	// progresses on the topics.
	//
	// In most cases, every instance is expected to have a unique ClientID.
	// The Kubernetes pod ID is usually a good candidate for this unique ID.
	ClientID string `yaml:"clientID"`

	// Optional. When GroupID is non-empty, a new group consumer will be created
	// instead. Messages from the topic will be consumed by one of the consumers
	// in the group (sharing the same GroupID) exactly once. This is the usual use
	// case of streaming consumers.
	//
	// When GroupID is empty, each consumer will have the whole view of the topic
	// (based on Offset), so that is usually for use cases like to deliver
	// configs/data through Kafka brokers.
	//
	// When GroupID is non-empty, Version must be at least "0.10.2.0".
	GroupID string `yaml:"groupID"`

	// Optional. The version of the kafka broker this consumer is connected to.
	// In format of "0.10.2.0" or "2.4.0".
	//
	// When omitted, Sarama library would pick the oldest supported version in
	// order to maintain maximum backward compatibility, but some of the newer
	// features might be unavailable. For example, using GroupID requires the
	// version to be at least "0.10.2.0".
	Version string `yaml:"version"`

	// Optional. Defaults to "oldest". Valid values are "oldest" and "newest".
	//
	// Only used when GroupID is empty.
	Offset string `yaml:"offset"`

	// Optional. If non-nil, will be used to log errors. At present, this only
	// pertains to logging errors closing the existing consumer when calling
	// consumer.reset() when GroupID is empty.
	Logger log.Wrapper `yaml:"logger"`
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

	var version sarama.KafkaVersion
	if cfg.Version != "" {
		var err error
		version, err = sarama.ParseKafkaVersion(cfg.Version)
		if err != nil {
			return nil, fmt.Errorf(
				"kafkabp: ParseKafkaVersion error: %w",
				err,
			)
		}
	}

	c := sarama.NewConfig()

	c.ClientID = cfg.ClientID
	c.Version = version

	if cfg.GroupID == "" {
		var offset int64
		switch cfg.Offset {
		case "", OffsetOldest:
			// OffsetOldest is the "true" default case (in that it will be reached if
			// an offset isn't specified).
			offset = sarama.OffsetOldest
		case OffsetNewest:
			offset = sarama.OffsetNewest
		default:
			return nil, ErrOffsetInvalid
		}
		c.Consumer.Offsets.Initial = offset
	}

	// Return any errors that occurred while consuming on the Errors channel.
	c.Consumer.Return.Errors = true

	return c, nil
}
