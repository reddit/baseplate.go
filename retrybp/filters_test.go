package retrybp_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/avast/retry-go"

	"github.com/reddit/baseplate.go/clientpool"
	"github.com/reddit/baseplate.go/retrybp"
)

func TestFilter(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		filters  []retrybp.Filter
		expected int
	}{
		{
			name:     "do-retry",
			filters:  []retrybp.Filter{doFilter},
			expected: maxAttempts,
		},
		{
			name:     "do-not-retry",
			filters:  []retrybp.Filter{doNotFilter},
			expected: 1,
		},
		{
			name:     "no-decision",
			filters:  []retrybp.Filter{noDecisionFilter},
			expected: 1,
		},
		{
			name: "do-not-retry-first",
			filters: []retrybp.Filter{
				doNotFilter,
				doFilter,
			},
			expected: 1,
		},
		{
			name: "do-retry-first",
			filters: []retrybp.Filter{
				doFilter,
				doNotFilter,
			},
			expected: maxAttempts,
		},
		{
			name: "no-decision-first",
			filters: []retrybp.Filter{
				noDecisionFilter,
				doFilter,
			},
			expected: maxAttempts,
		},
	}

	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				counter := &counter{}
				retrybp.Do(
					context.TODO(),
					counter.call,
					retry.Attempts(maxAttempts),
					retry.Delay(0),
					retry.DelayType(retry.FixedDelay),
					retrybp.Filters(c.filters...),
				)
				if counter.calls != c.expected {
					t.Errorf(
						"number of calls did not match, expected %v, got %v",
						c.expected,
						counter.calls,
					)
				}
			},
		)
	}
}

func TestContextErrorFilter(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		err      error
		expected int
	}{
		{
			name:     "unknown",
			err:      errors.New("test"),
			expected: maxAttempts,
		},
		{
			name:     "context.Canceled",
			err:      context.Canceled,
			expected: 1,
		},
		{
			name:     "context.DeadlineExceeded",
			err:      context.DeadlineExceeded,
			expected: 1,
		},
		{
			name:     "wrapped/context.Canceled",
			err:      fmt.Errorf("test: error. %w", context.Canceled),
			expected: 1,
		},
		{
			name:     "wrapped/context.DeadlineExceeded",
			err:      fmt.Errorf("test: error. %w", context.DeadlineExceeded),
			expected: 1,
		},
	}

	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				counter := &counter{err: c.err}
				retrybp.Do(
					context.TODO(),
					counter.call,
					retry.Attempts(maxAttempts),
					retry.Delay(0),
					retry.DelayType(retry.FixedDelay),
					retrybp.Filters(
						retrybp.ContextErrorFilter,
						doFilter,
					),
				)
				if counter.calls != c.expected {
					t.Errorf(
						"number of calls did not match, expected %v, got %v",
						c.expected,
						counter.calls,
					)
				}
			},
		)
	}
}

func TestPoolExhaustedFilter(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		err      error
		expected int
	}{
		{
			name:     "unknown",
			err:      errors.New("test"),
			expected: 1,
		},
		{
			name:     "clientpool.ErrExhausted",
			err:      clientpool.ErrExhausted,
			expected: maxAttempts,
		},
		{
			name:     "wrapped",
			err:      fmt.Errorf("test: error. %w", clientpool.ErrExhausted),
			expected: maxAttempts,
		},
	}

	for name, f := range map[string]retrybp.Filter{
		"RetryableErrorFilter": retrybp.RetryableErrorFilter,
		"PoolExhaustedFilter":  retrybp.PoolExhaustedFilter,
	} {
		t.Run(name, func(t *testing.T) {
			for _, _c := range cases {
				c := _c
				t.Run(
					c.name,
					func(t *testing.T) {
						counter := &counter{err: c.err}
						retrybp.Do(
							context.TODO(),
							counter.call,
							retry.Attempts(maxAttempts),
							retry.Delay(0),
							retry.DelayType(retry.FixedDelay),
							retrybp.Filters(
								f,
								doNotFilter,
							),
						)
						if counter.calls != c.expected {
							t.Errorf(
								"number of calls did not match, expected %v, got %v",
								c.expected,
								counter.calls,
							)
						}
					},
				)
			}
		})
	}
}

func TestNetworkErrorFilter(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		err      error
		expected int
	}{
		{
			name:     "unknown",
			err:      errors.New("test"),
			expected: 1,
		},
		{
			// See: https://github.com/reddit/baseplate.go/issues/257
			name:     "regression-257",
			err:      context.DeadlineExceeded,
			expected: 1,
		},
		{
			name:     "net.AddrError",
			err:      &net.AddrError{},
			expected: maxAttempts,
		},
		{
			name:     "net.DNSConfigError",
			err:      &net.DNSConfigError{},
			expected: maxAttempts,
		},
		{
			name:     "net.DNSError",
			err:      &net.DNSError{},
			expected: maxAttempts,
		},
		{
			name:     "net.InvalidAddrError",
			err:      net.InvalidAddrError("test"),
			expected: maxAttempts,
		},
		{
			name:     "net.OpError",
			err:      &net.OpError{},
			expected: maxAttempts,
		},
		{
			name:     "net.UnknownNetworkError",
			err:      net.UnknownNetworkError("test"),
			expected: maxAttempts,
		},
		{
			name:     "wrapped",
			err:      fmt.Errorf("test: error. %w", &net.AddrError{}),
			expected: maxAttempts,
		},
	}

	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				counter := &counter{err: c.err}
				retrybp.Do(
					context.TODO(),
					counter.call,
					retry.Attempts(maxAttempts),
					retry.Delay(0),
					retry.DelayType(retry.FixedDelay),
					retrybp.Filters(
						retrybp.NetworkErrorFilter,
						doNotFilter,
					),
				)
				if counter.calls != c.expected {
					t.Errorf(
						"number of calls did not match, expected %v, got %v",
						c.expected,
						counter.calls,
					)
				}
			},
		)
	}
}

func TestUnrecoverableErrorFilter(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		err      error
		expected int
	}{
		{
			name:     "unknown",
			err:      errors.New("test"),
			expected: maxAttempts,
		},
		{
			name:     "retry.Unrecoverable",
			err:      retry.Unrecoverable(errors.New("test")),
			expected: 1,
		},
		{
			name: "wrapped",
			err: fmt.Errorf(
				"test: error. %w",
				retry.Unrecoverable(errors.New("test")),
			),
			expected: maxAttempts,
		},
	}

	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				counter := &counter{err: c.err}
				retrybp.Do(
					context.TODO(),
					counter.call,
					retry.Attempts(maxAttempts),
					retry.Delay(0),
					retry.DelayType(retry.FixedDelay),
					retrybp.Filters(
						retrybp.UnrecoverableErrorFilter,
						doFilter,
					),
				)
				if counter.calls != c.expected {
					t.Errorf(
						"number of calls did not match, expected %v, got %v",
						c.expected,
						counter.calls,
					)
				}
			},
		)
	}
}

func BenchmarkFilters(b *testing.B) {
	attempts := uint(10)
	cases := []struct {
		numFilters int
	}{
		{numFilters: 1},
		{numFilters: 5},
		{numFilters: 10},
		{numFilters: 50},
		{numFilters: 100},
		{numFilters: 1000},
		{numFilters: 10000},
		{numFilters: 100000},
	}
	for _, c := range cases {
		filters := make([]retrybp.Filter, c.numFilters)
		for i := 0; i < c.numFilters-1; i++ {
			filters[i] = noDecisionFilter
		}
		filters[c.numFilters-1] = doFilter
		b.Run(
			fmt.Sprintf("Filters/%d", c.numFilters),
			func(b *testing.B) {
				counter := &counter{}
				retrybp.Do(
					context.TODO(),
					counter.call,
					retry.Attempts(attempts),
					retry.Delay(0),
					retry.DelayType(retry.FixedDelay),
					retrybp.Filters(filters...),
				)
				if counter.calls != int(attempts) {
					b.Errorf(
						"number of calls did not match, expected %v, got %v",
						attempts,
						counter.calls,
					)
				}
			},
		)
	}
}
