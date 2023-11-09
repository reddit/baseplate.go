package clientpool

import (
	"fmt"
)

// ErrExhausted is the error returned by Get when the pool is exhausted.
var ErrExhausted = exhaustedError{}

type exhaustedError struct{}

func (exhaustedError) Error() string {
	return "clientpool: exhausted"
}

// Retryable implements retrybp.RetryableError.
//
// It always returns 1 to indicate that it's retryable.
func (exhaustedError) Retryable() int {
	return 1
}

// ConfigError is the error type returned when trying to open a new
// client pool, but the configuration values passed in won't work.
type ConfigError struct {
	BestEffortInitialClients int
	RequiredInitialClients   int
	MinClients               int
	MaxClients               int
}

var _ error = (*ConfigError)(nil)

func (e *ConfigError) Error() string {
	return fmt.Sprintf(
		"clientpool: need requiredClients (%d) <= initialClients (%d) <= maxClients (%d), and minClients (%d) <= maxClients (%d)",
		e.RequiredInitialClients,
		e.BestEffortInitialClients,
		e.MaxClients,
		e.MinClients,
		e.MaxClients,
	)
}
