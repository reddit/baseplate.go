package kafkabp

import (
	"errors"

	"github.com/Shopify/sarama"
)

const (
	// OffsetOldest yields the oldest offset available on the broker for a
	// partition.
	OffsetOldest = sarama.OffsetOldest

	// OffsetNewest yields the offset that will be assigned to the next mesage
	// that will be produced to the partition.
	OffsetNewest = sarama.OffsetNewest
)

var (
	ErrClientIDEmpty = errors.New("kafkabp: ClientID is empty")
	ErrBrokersEmpty  = errors.New("kafkabp: Brokers are empty")
	ErrTopicEmpty    = errors.New("kafkabp: Topic is empty")
)

// DefaultSaramaConfig returns the sane default Sarama configuration.
func DefaultSaramaConfig() *sarama.Config {
	return sarama.NewConfig()
}

func NewSaramaConsumer(brokers []string, cfg *sarama.Config) (sarama.Consumer, error) {
	return sarama.NewConsumer(brokers, cfg)
}
