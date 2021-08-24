package retrybp

import (
	"context"
	"errors"
	"net"

	retry "github.com/avast/retry-go"
	"github.com/sony/gobreaker"
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

// RetryableError defines an optional error interface to return retryable info.
type RetryableError interface {
	error

	// Errors should return 0 if there's not enough information to make a
	// decision, or >0 to indicate that it's retryable, and <0 means it's not.
	Retryable() int
}

// Any thrift exception with an optional boolean field named "retryable" would
// generate go code that implements this interface.
//
// It's unexported as future thrift compiler changes could change the actual
// function name and as a result we don't want to make it part of our public
// API.
type thriftRetryableError interface {
	error

	IsSetRetryable() bool
	GetRetryable() bool
}

// RetryableErrorFilter is a Filter implementation that checks RetryableError.
//
// If err is not an implementation of RetryableError, or if its Retryable()
// returns nil, it defers to the next filter.
// Otherwise it use the Retryable() result.
//
// It also checks against thrift exceptions with an optional boolean field named
// "retryable" defined, and use that field as the decision (unset means no
// decision).
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
	var tre thriftRetryableError
	if errors.As(err, &re) {
		if v := re.Retryable(); v != 0 {
			return v > 0
		}
	} else if errors.As(err, &tre) {
		if tre.IsSetRetryable() {
			return tre.GetRetryable()
		}
	} else if !retry.IsRecoverable(err) {
		// In case users are mistakenly using retry.Unrecoverable instead of
		// retrybp.Unrecoverable.
		return false
	}
	return next(err)
}

// BreakerErrorFilter is a Filter implementations that retries circuit-breaker
// errors from gobreaker/breakerbp.
//
// It should only be used when you are using circuit breaker,
// and have proper backoff policy set
// (e.g. using retrybp.CappedExponentialBackoff).
func BreakerErrorFilter(err error, next retry.RetryIfFunc) bool {
	// ErrOpenState is returned when the breaker is in open state,
	// and ErrTooManyRequests is returned when the breaker is half-open and rate
	// limited.
	//
	// They are both retryable as long as we have proper backoff policy set,
	// as if the state is still open when we retry it will fail fast and also not
	// adding extra load to the upstream service.
	if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
		return true
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
	_ Filter = RetryableErrorFilter
	_ Filter = BreakerErrorFilter

	_ RetryableError = retryableWrapper{}
)
