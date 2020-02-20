package clientpool

import (
	"errors"
	"fmt"
)

// ErrExhausted is the error returned by Get when the pool is exhausted.
var ErrExhausted = errors.New("client pool exhausted")

// ConfigError is the error type returned when trying to open a new thrift
// client pool, but the configuration values passed in won't work.
type ConfigError struct {
	MinConnections int
	MaxConnections int
}

var _ error = (*ConfigError)(nil)

func (e *ConfigError) Error() string {
	return fmt.Sprintf(
		"clientpool: minConnections (%d) > maxConnections (%d)",
		e.MinConnections,
		e.MaxConnections,
	)
}
