package kafkabp

import (
	"errors"
	"fmt"

	"github.com/Shopify/sarama"
	"github.com/prometheus/client_golang/prometheus"

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

	// ErrNilConsumePartitionFunc is thrown when ConsumePartitionFuncProvider
	// returns a nil ConsumePartitionFunc.
	ErrNilConsumePartitionFunc = errors.New("kafkabp: ConsumePartitionFunc is nil")
)

// ConsumerConfig can be used to configure a kafkabp Consumer.
//
// Can be deserialized from YAML.
//
// Example:
//
//	kafka:
//	  brokers:
//	    - 127.0.0.1:9090
//	    - 127.0.0.2:9090
//	  topic: sample-topic
//	  clientID: myclient
//	  version: 2.4.0
//	  offset: oldest
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

	// Optional. This is only applicable when GroupID is empty string.
	// When GroupID is empty, the configuration enables a TopicConsumer to read
	// from all partitions of the given kafka stream.
	// When ConsumePartitionFuncProvider is also specified, the TopicConsumer will
	// only consume partitions that evaluates to true with the given
	// predicate returned from ConsumePartitionFuncProvider.
	// If ConsumePartitionFuncProvider is specified,
	// it must return a non-nil predicate of type ConsumePartitionFunc,
	// else the topic consumer will return error ErrNilConsumePartitionFunc.
	//
	// This function is called once per reset. The returned ConsumePartitionFunc
	// is also called once per reset per partition.
	// The API is designed to be two-layer so that it's possible to shift all
	// heavylifting to ConsumePartitionFuncProvider to generate a []bool or set of
	// ints, and make the returned ConsumePartitionFunc just do a simple lookup.
	ConsumePartitionFuncProvider ConsumePartitionFuncProvider `yaml:"-"`

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

	// Optional. The function to set rack id for this kafka client.
	// It should match rack configured on the broker(s).
	//
	// Currently it defaults to no rack id.
	// In the future the default might be changed to AWSAvailabilityZoneRackID.
	//
	// This feature is currently experimental.
	// It might not make any difference on your client,
	// or it might make things worse.
	// You are advised to test before using non-empty rack id in production.
	RackID RackIDFunc `yaml:"rackID"`
}

// Since not all sarama's default config are zero values,
// this gives us an easy access to them.
var defaultSaramaConfig = sarama.NewConfig()

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

	version := defaultSaramaConfig.Version
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
	kafkaVersionGauge.With(prometheus.Labels{
		"kafka_version": version.String(),
	}).Set(1)

	c := sarama.NewConfig()

	// Return any errors that occurred to the Errors channel.
	c.Consumer.Return.Errors = true

	c.ClientID = cfg.ClientID
	c.Version = version

	if cfg.RackID != nil {
		c.RackID = cfg.RackID()
	}

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

	return c, nil
}
