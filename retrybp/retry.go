package retrybp

import (
	"context"
	"errors"
	"time"

	retry "github.com/avast/retry-go"

	"github.com/reddit/baseplate.go/errorsbp"
)

func init() {
	retry.DefaultAttempts = 1
	retry.DefaultDelay = 1 * time.Millisecond
	retry.DefaultMaxJitter = 5 * time.Millisecond
	retry.DefaultDelayType = cappedExponentialBackoffFunc(CappedExponentialBackoffArgs{
		InitialDelay: retry.DefaultDelay,
		MaxJitter:    retry.DefaultMaxJitter,
	})
	retry.DefaultLastErrorOnly = false
}

type contextKeyType struct{}

var contextKey contextKeyType

// WithOptions sets the given retry.Options on the given context.
func WithOptions(ctx context.Context, options ...retry.Option) context.Context {
	return context.WithValue(ctx, contextKey, options)
}

// GetOptions returns the list of retry.Options set on the context.
func GetOptions(ctx context.Context) (options []retry.Option, ok bool) {
	options, ok = ctx.Value(contextKey).([]retry.Option)
	return
}

// Do retries the given function using retry.Do with the default retry.Options
// provided and overriding them with any options set on the context via
// WithOptions.
//
// The changes this has compared to retry.Do are:
//
// 1. Pulling options from the context.  This allows it to be used in middleware
// where you are not calling Do directly but still want to be able to configure
// retry behavior per-call.
//
// 2. If retry.Do returns a batch of errors (retry.Error), put those in a
// errorsbp.Batch from baseplate.go.
//
// It also auto applies retry.Context with the ctx given,
// so that the retries will be stopped as soon as ctx is canceled.
// You can override this behavior by injecting a retry.Context option into ctx.
func Do(ctx context.Context, fn func() error, defaults ...retry.Option) error {
	options, _ := GetOptions(ctx)
	mergedOptions := make([]retry.Option, 1, 1+len(defaults)+len(options))
	// Always do this as the first option, to allow overriding it in either
	// defaults or options from the ctx.
	mergedOptions[0] = retry.Context(ctx)
	mergedOptions = append(mergedOptions, defaults...)
	mergedOptions = append(mergedOptions, options...)
	err := retry.Do(fn, mergedOptions...)

	var retryErr retry.Error
	if errors.As(err, &retryErr) {
		var batchErr errorsbp.Batch
		batchErr.Add(retryErr.WrappedErrors()...)
		return batchErr.Compile()
	}

	return err
}
