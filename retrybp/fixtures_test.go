package retrybp_test

import (
	"fmt"

	"github.com/avast/retry-go"
)

const (
	maxAttempts = 5
)

func doFilter(err error, next retry.RetryIfFunc) bool {
	return true
}

func doNotFilter(err error, next retry.RetryIfFunc) bool {
	return false
}

func noDecisionFilter(err error, next retry.RetryIfFunc) bool {
	return next(err)
}

type counter struct {
	calls int
	err   error
}

func (c *counter) call() error {
	c.calls++
	if c.err != nil {
		return c.err
	}
	return fmt.Errorf("counter: %d", c.calls)
}
