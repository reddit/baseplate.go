package clientpool

import (
	"errors"
	"fmt"
)

// ErrExhausted is the error returned by Get when the pool is exhausted.
var ErrExhausted = errors.New("clientpool: exhausted")

// ConfigError is the error type returned when trying to open a new
// client pool, but the configuration values passed in won't work.
type ConfigError struct {
	InitialClients int
	MaxClients     int
}

var _ error = (*ConfigError)(nil)

func (e *ConfigError) Error() string {
	return fmt.Sprintf(
		"clientpool: initialClients (%d) > maxClients (%d)",
		e.InitialClients,
		e.MaxClients,
	)
}
