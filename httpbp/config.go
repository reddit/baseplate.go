package httpbp

import (
	retry "github.com/avast/retry-go"
	"github.com/reddit/baseplate.go/breakerbp"
	"github.com/reddit/baseplate.go/errorsbp"
)

// ClientConfig provides the configuration for a HTTP client including its
// middlewares.
type ClientConfig struct {
	Slug              string            `yaml:"slug"`
	MaxErrorReadAhead int               `yaml:"limitErrorReading"`
	MaxConnections    int               `yaml:"maxConnections"`
	CircuitBreaker    *breakerbp.Config `yaml:"circuitBreaker"`
	RetryOptions      []retry.Option
}

// Validate checks ClientConfig for any missing or erroneous values.
func (c ClientConfig) Validate() error {
	var batch errorsbp.Batch
	if c.Slug == "" {
		batch.Add(ErrConfigMissingSlug)
	}
	if c.MaxErrorReadAhead < 0 {
		batch.Add(ErrConfigInvalidMaxErrorReadAhead)
	}
	if c.MaxConnections < 0 {
		batch.Add(ErrConfigInvalidMaxConnections)
	}
	return batch.Compile()
}
