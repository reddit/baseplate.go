package httpbp

import (
	"errors"

	"github.com/avast/retry-go"

	"github.com/reddit/baseplate.go/breakerbp"
)

// ClientConfig provides the configuration for a HTTP client including its
// middlewares.
type ClientConfig struct {
	Slug              string            `yaml:"slug"`
	MaxErrorReadAhead int               `yaml:"limitErrorReading"`
	MaxConnections    int               `yaml:"maxConnections"`
	CircuitBreaker    *breakerbp.Config `yaml:"circuitBreaker"`
	RetryOptions      []retry.Option

	SecretsStore           SecretsStore
	HeaderbpSigningKeyPath string

	// deprecated
	InternalOnly bool
}

// Validate checks ClientConfig for any missing or erroneous values.
func (c ClientConfig) Validate() error {
	var errs []error
	if c.Slug == "" {
		errs = append(errs, ErrConfigMissingSlug)
	}
	if c.MaxErrorReadAhead < 0 {
		errs = append(errs, ErrConfigInvalidMaxErrorReadAhead)
	}
	if c.MaxConnections < 0 {
		errs = append(errs, ErrConfigInvalidMaxConnections)
	}
	return errors.Join(errs...)
}
