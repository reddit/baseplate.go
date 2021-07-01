package kafkabp_test

import (
	"errors"
	"testing"

	"github.com/reddit/baseplate.go/kafkabp"
)

func TestConsumerConfig(t *testing.T) {
	var cfg kafkabp.ConsumerConfig

	t.Run("no-brokers", func(t *testing.T) {
		sc, err := cfg.NewSaramaConfig()
		if sc != nil {
			t.Errorf("expected config to be nil, got %v", sc)
		}
		if !errors.Is(err, kafkabp.ErrBrokersEmpty) {
			t.Errorf("expected error %v, got %v", kafkabp.ErrBrokersEmpty, err)
		}
	})
	cfg.Brokers = []string{"127.0.0.1:9090", "127.0.0.2:9090"}

	t.Run("no-topic", func(t *testing.T) {
		sc, err := cfg.NewSaramaConfig()
		if sc != nil {
			t.Errorf("expected config to be nil, got %v", sc)
		}
		if !errors.Is(err, kafkabp.ErrTopicEmpty) {
			t.Errorf("expected error %v, got %v", kafkabp.ErrTopicEmpty, err)
		}
	})
	cfg.Topic = "test-topic"

	t.Run("no-client-id", func(t *testing.T) {
		sc, err := cfg.NewSaramaConfig()
		if sc != nil {
			t.Errorf("expected config to be nil, got %v", sc)
		}
		if !errors.Is(err, kafkabp.ErrClientIDEmpty) {
			t.Errorf("expected error %v, got %v", kafkabp.ErrClientIDEmpty, err)
		}
	})
	cfg.ClientID = "i am unique"

	t.Run("invalid-offset", func(t *testing.T) {
		cfg.Offset = "fanciest"
		sc, err := cfg.NewSaramaConfig()
		if sc != nil {
			t.Errorf("expected config to be nil, got %v", sc)
		}
		if !errors.Is(err, kafkabp.ErrOffsetInvalid) {
			t.Errorf("expected error %v, got %v", kafkabp.ErrOffsetInvalid, err)
		}
	})
	cfg.Offset = "newest"

	t.Run("invalid-version", func(t *testing.T) {
		cfg.Version = "foo"
		sc, err := cfg.NewSaramaConfig()
		if sc != nil {
			t.Errorf("expected config to be nil, got %v", sc)
		}
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
	cfg.Version = "2.5.0"

	t.Run("valid-config", func(t *testing.T) {
		sc, err := cfg.NewSaramaConfig()
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if sc == nil {
			t.Fatal("expected config to be non-nil, got nil")
		}
		if sc.ClientID != cfg.ClientID {
			t.Errorf("expected sarama client id to be %q, got %q", cfg.ClientID, sc.ClientID)
		}
	})

	const rackID = "foo"
	cfg.RackID = kafkabp.FixedRackID(rackID)
	t.Run("rack-id", func(t *testing.T) {
		sc, err := cfg.NewSaramaConfig()
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if sc == nil {
			t.Fatal("expected config to be non-nil, got nil")
		}
		if sc.RackID != rackID {
			t.Errorf("expected sarama rack id to be %q, got %q", rackID, sc.ClientID)
		}
	})

	cfg.RackID = nil
	t.Run("no-rack-id", func(t *testing.T) {
		sc, err := cfg.NewSaramaConfig()
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if sc == nil {
			t.Fatal("expected config to be non-nil, got nil")
		}
		if sc.RackID != "" {
			t.Errorf("expected sarama rack id to be empty, got %q", sc.ClientID)
		}
	})
}
