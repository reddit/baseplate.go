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
//
// DEPRECATED: clientpool.ErrExhausted implements RetryableError,
// so RetryableErrorFilter covers the functionality of this filter and should be
// used instead.
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
//
// DEPRECATED: Please use retrybp.Unrecoverable and RetryableErrorFilter instead.
func UnrecoverableErrorFilter(err error, next retry.RetryIfFunc) bool {
	return RetryableErrorFilter(err, next)
}

// RetryableError defines an optional error interface to return retryable info.
type RetryableError interface {
	error

	// Errors should return 0 if there's not enough information to make a
	// decision, or >0 to indicate that it's retryable, and <0 means it's not.
	Retryable() int
}

// RetryableErrorFilter is a Filter implementation that checks RetryableError.
//
// If err is not an implementation of RetryableError, or if its Retryable()
// returns nil, it defers to the next filter.
// Otherwise it use the Retryable() result.
//
// In addition, it also checks retry.IsRecoverable, in case retry.Unrecoverable
// was used instead of retrybp.Unrecoverable.
//
// In most cases this should be the first in the filter chain,
// because functions could use Unrecoverable to wrap errors that would return
// true in other filter implementations to explicitly override those filter
// behaviors.
func RetryableErrorFilter(err error, next retry.RetryIfFunc) bool {
	var re RetryableError
	if errors.As(err, &re) {
		if v := re.Retryable(); v != 0 {
			return v > 0
		}
	} else if !retry.IsRecoverable(err) {
		// In case users are mistakenly using retry.Unrecoverable instead of
		// retrybp.Unrecoverable.
		return false
	}
	return next(err)
}

type retryableWrapper struct {
	err       error
	retryable int
}

func (e retryableWrapper) Error() string {
	return e.err.Error()
}

func (e retryableWrapper) Unwrap() error {
	return e.err
}

func (e retryableWrapper) Retryable() int {
	return e.retryable
}

// Unrecoverable wraps an error and mark it as unrecoverable by implementing
// RetryableError and returning false on Retryable().
//
// It's similar to retry.Unrecoverable,
// but properly implements error unwrapping API in go 1.13+.
// As a result, it's preferred over retry.Unrecoverable.
func Unrecoverable(err error) error {
	if err == nil {
		return nil
	}
	return retryableWrapper{
		err:       err,
		retryable: -1,
	}
}

var (
	_ Filter = ContextErrorFilter
	_ Filter = NetworkErrorFilter
	_ Filter = PoolExhaustedFilter
	_ Filter = UnrecoverableErrorFilter
	_ Filter = RetryableErrorFilter

	_ RetryableError = retryableWrapper{}
)
