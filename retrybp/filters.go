package retrybp

import (
	"context"
	"errors"
	"net"

	retry "github.com/avast/retry-go"

	"github.com/reddit/baseplate.go/clientpool"
)

const (
	// DefaultFilterDecision is the default decision returned by Filters at the end
	// of the filter chain.
	DefaultFilterDecision = false
)

func fallback(_ error) bool {
	return DefaultFilterDecision
}

var _ retry.RetryIfFunc = fallback

func chain(current Filter, next retry.RetryIfFunc) retry.RetryIfFunc {
	return func(err error) bool {
		return current(err, next)
	}
}

// Filters returns a `retry.RetryIf` function that checks the error against the
// given filters and returns either the decision reached by a filter or the
// DefaultFilterDecision.
//
// You should not use this with any other retry.RetryIf options as one will
// override the other.
func Filters(filters ...Filter) retry.Option {
	retryIf := fallback
	for i := len(filters) - 1; i >= 0; i-- {
		retryIf = chain(filters[i], retryIf)
	}
	return retry.RetryIf(retryIf)
}

// Filter is a function that is passed an error and attempts to determine
// if the request should be retried given the error.
//
// Filters should only implement a single check that will generally
// determine whether the request should be retried or not.  If it cannot make
// that decision on it's own, it should call the next RetryIfFunc in the chain.
//
// If a filter is doing more than that, there's a good chance that it is doing
// too much.
type Filter func(err error, next retry.RetryIfFunc) bool

// ContextErrorFilter returns false if the error is context.Cancelled
// or context.DeadlineExceeded, otherwise it calls the next filter in the chain.
func ContextErrorFilter(err error, next retry.RetryIfFunc) bool {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return false
	}

	return next(err)
}

// NetworkErrorFilter returns true if the error is a net.Error error otherwise it
// calls the next filter in the chain.
//
// This filter assumes that the error is due to a problem with the specific
// connection that was used to make the requset and using a new connection would
// fix the problem, which is why it retries on every net.Error error rather than
// inspecting the error for more details to determine if it is appropriate to
// retry or not.
//
// This should only be used for idempotent requests.  You don't want to retry
// calls that have side effects if the network connection broke down sometime
// between sending and receiving since you have no way of knowing if the callee
// receieved and is already processing your request.
func NetworkErrorFilter(err error, next retry.RetryIfFunc) bool {
	if !errors.Is(err, context.DeadlineExceeded) && errors.As(err, new(net.Error)) {
		return true
	}
	return next(err)
}

// PoolExhaustedFilter returns true if the error is an
// clientpool.ErrExhausted error otherwise it calls the next filter in the chain.
//
// This is safe to use even if a request is not idempotent as this error happens
// before any network calls are made.  It is best paired with some backoff
// though to give the pool some time to recover.
func PoolExhaustedFilter(err error, next retry.RetryIfFunc) bool {
	if errors.Is(err, clientpool.ErrExhausted) {
		return true
	}
	return next(err)
}

// UnrecoverableErrorFilter returns false if the error is an
// retry.Unrecoverable error otherwise it calls the next filter in the chain.
//
// This uses retry.IsRecoverable which relies on wrapping the error with
// retry.Unrecoverable.  It also does not use the "errors" helpers so if the
// the error returned by retry.Unrecoverable is further wrapped, this will not
// be able to make a decision.
func UnrecoverableErrorFilter(err error, next retry.RetryIfFunc) bool {
	if !retry.IsRecoverable(err) {
		return false
	}
	return next(err)
}

var (
	_ Filter = ContextErrorFilter
	_ Filter = NetworkErrorFilter
	_ Filter = PoolExhaustedFilter
	_ Filter = UnrecoverableErrorFilter
)
