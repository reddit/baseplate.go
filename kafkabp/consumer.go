package kafkabp

import (
	"fmt"
	"log"
	"os"

	"github.com/Shopify/sarama"
)

func main() {
	fmt.Println("vim-go")

	sarama.Logger = log.New(os.Stderr, "[sarama] ", log.LstdFlags)
}

type consumer struct {
	cfg     ConsumerConfig
	topic   string
	offset  int64
	tracing bool
}

type Consumer interface {
}

// NewConsumer creates a new Kafka consumer.
func NewConsumer(cfg ConsumerConfig) (Consumer, error) {
	// Validate input parameters.
	if cfg.SaramaConfig == nil {
		cfg.SaramaConfig = DefaultSaramaConfig()
	}
	if cfg.ClientID != "" {
		cfg.SaramaConfig.ClientID = cfg.ClientID
	}
	if cfg.SaramaConfig.ClientID == "" {
		return nil, ErrClientIDEmpty
	}

	if len(cfg.Brokers) == 0 {
		return nil, ErrBrokersEmpty
	}

	if cfg.Topic == "" {
		return nil, ErrTopicEmpty
	}

	if cfg.Offset != OffsetNewest {
		cfg.Offset = OffsetOldest
	}

	// Return any errors that occurred while consuming on the Errors channel.
	cfg.SaramaConfig.Consumer.Return.Errors = true

	kc := &consumer{
		cfg:     cfg,
		topic:   cfg.Topic,
		offset:  int64(cfg.Offset),
		tracing: cfg.Tracing,
	}

	//if err := kc.resetConsumers(); err != nil {
	//	return nil, err
	//}

	return kc, nil
}

func (c *consumer) resetConsumers() error {
	return nil
}
