package kafkabp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig(t *testing.T) {
	var cfg ConsumerConfig
	// Config with invalid Offset should not create a new consumer and throw
	// ErrOffsetInvalid
	cfg.Offset = "fanciest"
	sc, err := cfg.NewSaramaConfig()
	assert.Nil(t, sc)
	assert.Equal(t, ErrOffsetInvalid, err)
}
