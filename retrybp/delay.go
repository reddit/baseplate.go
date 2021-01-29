package retrybp

import (
	"errors"
	"math"
	"time"

	retry "github.com/avast/retry-go"

	"github.com/reddit/baseplate.go/randbp"
)

// RetryAfterError defines a type of errors that contain retry-after
// information (for example, HTTP's Retry-After header).
//
// httpbp.ClientError is an error type that implements this interface.
type RetryAfterError interface {
	error

	// If RetryAfterDuration returns a duration <= 0,
	// it's considered as not having retry-after info.
	RetryAfterDuration() time.Duration
}

// CappedExponentialBackoffArgs defines the args used in
// CappedExponentialBackoff retry option.
//
// All args are optional.
type CappedExponentialBackoffArgs struct {
	// The initial delay.
	// If <=0, retry.DefaultDelay will be used.
	// If retry.DefaultDelay <= 0, 1 nanosecond will be used.
	InitialDelay time.Duration

	// The cap of InitialDelay<<n. If <=0, it will only be capped at MaxExponent.
	//
	// Please note that it doesn't cap the MaxJitter part,
	// so the actual max delay could be MaxDelay+MaxJitter.
	MaxDelay time.Duration

	// We calculate the delay before jitter by using InitialDelay<<n
	// (n being the number of retries). MaxExponent caps the n part.
	//
	// If <=0, it will be calculated based on InitialDelay to make sure that it
	// won't overflow signed int64.
	// If it's set to a value too high that would overflow,
	// it will also be adjusted automatically.
	//
	// Please note that MaxExponent doesn't limit the number of actual retries.
	// It only caps the number of retries used in delay value calculation.
	MaxExponent int

	// Max random jitter to be added to each retry delay.
	// If <=0, no random jitter will be added.
	MaxJitter time.Duration

	// When IgnoreRetryAfterError is set to false (default),
	// and the error caused the retry implements RetryAfterError,
	// and the returned RetryAfterDuration > 0,
	// it's guaranteed that the delay value is at least RetryAfterDuration + jitter.
	//
	// If the returned RetryAfterDuration conflicts with (is larger than) MaxDelay
	// or the calculated delay result from MaxExponent,
	// RetryAfterDuration takes priority.
	IgnoreRetryAfterError bool
}

// CappedExponentialBackoff is an exponentially backoff delay implementation
// that makes sure the delays are properly capped.
func CappedExponentialBackoff(args CappedExponentialBackoffArgs) retry.Option {
	return retry.DelayType(cappedExponentialBackoffFunc(args))
}

func cappedExponentialBackoffFunc(args CappedExponentialBackoffArgs) retry.DelayTypeFunc {
	base := args.InitialDelay
	if base <= 0 {
		base = retry.DefaultDelay
	}
	if base <= 0 {
		base = 1
	}

	maxExponent := actualMaxExponent(base)
	if args.MaxExponent > 0 && args.MaxExponent < maxExponent {
		maxExponent = args.MaxExponent
	}
	uMaxExponent := uint(maxExponent)

	maxInt64 := uint64(math.MaxInt64)

	return func(n uint, err error, _ *retry.Config) time.Duration {
		if n > uMaxExponent {
			n = uMaxExponent
		}
		delay := uint64(base) << n
		if args.MaxDelay > 0 && delay > uint64(args.MaxDelay) {
			delay = uint64(args.MaxDelay)
		}

		var rae RetryAfterError
		if !args.IgnoreRetryAfterError && errors.As(err, &rae) {
			if minDelay := rae.RetryAfterDuration(); minDelay > 0 && delay < uint64(minDelay) {
				delay = uint64(minDelay)
			}
		}

		if args.MaxJitter > 0 {
			delay += uint64(randbp.R.Int63n(int64(args.MaxJitter)))
		}
		// Although base << maxExponent won't overflow signed int64,
		// adding jitter might overflow it.
		if delay > maxInt64 {
			delay = maxInt64
		}

		return time.Duration(delay)
	}
}

func actualMaxExponent(base time.Duration) int {
	if base <= 0 {
		base = 1
	}
	// 1 << 63 would overflow signed int64, thus 62.
	return 62 - int(math.Floor(math.Log2(float64(base))))
}

// FixedDelay is a delay option to use fixed delay between retries.
//
// To achieve the same result via upstream retry package's API,
// you would need to combine retry.Delay and retry.DelayType(retry.FixedDelay),
// which is confusing and error-prone.
// As a result we provide this API to make things easier.
//
// If you want to combine FixedDelay with a random jitter,
// you could use FixedDelayFunc with retry.RandomDelay, example:
//
//     retry.DelayType(retry.CombineDelay(retry.RandomDelay, retrybp.FixedDelayFunc(delay)))
func FixedDelay(delay time.Duration) retry.Option {
	return retry.DelayType(FixedDelayFunc(delay))
}

// FixedDelayFunc is an retry.DelayTypeFunc implementation causing fixed delays.
func FixedDelayFunc(delay time.Duration) retry.DelayTypeFunc {
	return func(_ uint, _ error, _ *retry.Config) time.Duration {
		return delay
	}
}
