package retrybp_test

import (
	"context"
	"time"

	"github.com/avast/retry-go"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/reddit/baseplate.go/breakerbp"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/retrybp"
)

// This example demonstrates how to use retrybp to retry an operation.
func Example() {
	const timeout = time.Millisecond * 200

	// prometheus counters
	var (
		errorCounter   prometheus.Counter
		successCounter prometheus.Counter
	)

	// TODO: use the actual type of your operation.
	type resultType = int

	// In real code this should be your actual operation you need to retry.
	// Here we just use a placeholder for demonstration purpose.
	operation := func() (resultType, error) {
		return 0, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	// Optional: only do this if you want to add specific, per-call retry options.
	ctx = retrybp.WithOptions(
		ctx,
		// Do 3 retries, and override whatever retries is defined in retrybp.Do.
		retry.Attempts(3+1),
	)

	const defaultRetries = 2
	var result resultType
	err := retrybp.Do(
		ctx,
		func() error {
			// This is the main function call
			var err error
			result, err = operation()
			return err
		},

		// Default retry options, can be overridden by ctx passed in.
		retry.Attempts(defaultRetries+1), // retries + the first attempt
		retrybp.CappedExponentialBackoff(retrybp.CappedExponentialBackoffArgs{
			InitialDelay: time.Millisecond * 10,
			MaxJitter:    time.Millisecond * 5,
		}),
		retrybp.Filters(
			retrybp.RetryableErrorFilter, // this should always be the first filter.
			retrybp.ContextErrorFilter,
			retrybp.NetworkErrorFilter, // optional: only use this if the requests are idempotent.

			// By default, if none of the filters above can make a decision,
			// the final decision would be not to retry (see retrybp.DefaultFilterDecision),
			// here we override it to always retry instead.
			//
			// This is only for demonstration purpose.
			// in general you want to check whether an error is retry-able,
			// instead of doing this blind decision.
			func(_ error, _ retry.RetryIfFunc) bool {
				return true
			},
		),
		retry.OnRetry(func(attempts uint, err error) {
			if err != nil {
				errorCounter.Inc()
				log.Errorw(
					"operation failed",
					"err", err,
					"attempt", attempts,
				)
			} else {
				successCounter.Inc()
			}
		}),
	)

	// TODO: In real code, you need to check error and use the result here.
	_ = err
	_ = result
}

// This example demonstrates how to use retrybp with breakerbp.
func ExampleBreakerErrorFilter() {
	const timeout = time.Millisecond * 200

	// TODO: use the actual type of your operation.
	type resultType = int

	// In real code this should be your actual operation you need to retry.
	// Here we just use a placeholder for demonstration purpose.
	operation := func() (resultType, error) {
		return 0, nil
	}

	breaker := breakerbp.NewFailureRatioBreaker(breakerbp.Config{
		// TODO: Add breaker configs.
	})

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	const defaultRetries = 2
	var result resultType
	err := retrybp.Do(
		ctx,
		func() error {
			// This is the main function call,
			// wrapped with breaker.Execute to apply circuit breaker on it.
			_, err := breaker.Execute(func() (interface{}, error) {
				var err error
				result, err = operation()
				return result, err
			})
			return err
		},

		// Retry options.
		retry.Attempts(defaultRetries+1), // retries + the first attempt
		retrybp.CappedExponentialBackoff(retrybp.CappedExponentialBackoffArgs{
			InitialDelay: time.Millisecond * 10,
			MaxJitter:    time.Millisecond * 5,
		}),
		retrybp.Filters(
			retrybp.RetryableErrorFilter, // this should always be the first filter.
			retrybp.BreakerErrorFilter,   // this is the filter to handle errors returned by the breaker.
			retrybp.ContextErrorFilter,
			retrybp.NetworkErrorFilter, // optional: only use this if the requests are idempotent.
		),
	)

	// TODO: In real code, you need to check error and use the result here.
	_ = err
	_ = result
}
