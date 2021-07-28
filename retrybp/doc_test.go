package retrybp_test

import (
	"context"
	"time"

	"github.com/avast/retry-go"

	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/metricsbp"
	"github.com/reddit/baseplate.go/retrybp"
)

// This example demonstrates how to use retrybp to retry an operation.
func Example() {
	const timeout = time.Millisecond * 200

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
		retry.OnRetry(func(n uint, err error) {
			if err != nil {
				metricsbp.M.Counter("operation.failure").Add(1)
				log.Errorw(
					"operation failed",
					"err", err,
					"attempt", n,
				)
			} else {
				metricsbp.M.Counter("operation.succeed").Add(1)
			}
		}),
	)

	// TODO: check error
	_ = err
	// TODO: use result
	_ = result
}
