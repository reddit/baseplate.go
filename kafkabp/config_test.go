package kafkabp

import (
	"errors"
	"testing"
)

func TestConfig(t *testing.T) {
	var cfg ConsumerConfig
	// Config with invalid Offset should not create a new consumer and throw
	// ErrOffsetInvalid
	cfg.Offset = "fanciest"
	sc, err := cfg.NewSaramaConfig()
	if sc != nil {
		t.Errorf("expected Sarama config to be nil, got %v", sc)
	}
	if !errors.Is(err, ErrOffsetInvalid) {
		t.Errorf("expected error %v, got %v", ErrOffsetInvalid, err)
	}
}
