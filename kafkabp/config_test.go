package kafkabp

import (
	"errors"
	"testing"
)

func TestConfig(t *testing.T) {
	var cfg ConsumerConfig

	// Config with no Brokers should not create a new Sarama config and throw
	// ErrBrokersEmpty
	sc, err := cfg.NewSaramaConfig()
	if sc != nil {
		t.Errorf("expected config to be nil, got %v", sc)
	}
	if !errors.Is(err, ErrBrokersEmpty) {
		t.Errorf("expected error %v, got %v", ErrBrokersEmpty, err)
	}

	// Config with no Topic should not create a new consumer and throw
	// ErrTopicEmpty
	cfg.Brokers = []string{"127.0.0.1:9090", "127.0.0.2:9090"}
	sc, err = cfg.NewSaramaConfig()
	if sc != nil {
		t.Errorf("expected config to be nil, got %v", sc)
	}
	if !errors.Is(err, ErrTopicEmpty) {
		t.Errorf("expected error %v, got %v", ErrTopicEmpty, err)
	}

	// Config with no ClientID should not create a new consumer and throw
	// ErrClientIDEmpty
	cfg.Topic = "test-topic"
	sc, err = cfg.NewSaramaConfig()
	if sc != nil {
		t.Errorf("expected config to be nil, got %v", sc)
	}
	if !errors.Is(err, ErrClientIDEmpty) {
		t.Errorf("expected error %v, got %v", ErrClientIDEmpty, err)
	}

	// Config with invalid Offset should not create a new consumer and throw
	// ErrOffsetInvalid
	cfg.ClientID = "i am unique"
	cfg.Offset = "fanciest"
	sc, err = cfg.NewSaramaConfig()
	if sc != nil {
		t.Errorf("expected config to be nil, got %v", sc)
	}
	if !errors.Is(err, ErrOffsetInvalid) {
		t.Errorf("expected error %v, got %v", ErrOffsetInvalid, err)
	}
}
