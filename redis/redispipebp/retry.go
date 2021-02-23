package redispipebp

import (
	"context"
	"errors"

	"github.com/avast/retry-go"
	"github.com/joomcode/redispipe/redis"
	"github.com/reddit/baseplate.go/retrybp"

	"github.com/reddit/baseplate.go/redis/redisx"
)

// RetrySync wraps redis calls in retry logic using the given Options.
type RetrySync struct {
	Sync        redisx.Sync
	DefaultOpts []retry.Option
}

// Do wraps s.Sync.Do in the retry logic.
func (s RetrySync) Do(ctx context.Context, cmd string, args ...interface{}) (result interface{}) {
	_ = retrybp.Do(ctx, func() error {
		result = s.Sync.Do(ctx, cmd, args...)
		return redis.AsError(result)
	}, s.DefaultOpts...)
	return
}

// Send wraps s.Sync.Send in the retry logic.
func (s RetrySync) Send(ctx context.Context, req redis.Request) (result interface{}) {
	_ = retrybp.Do(ctx, func() error {
		result = s.Sync.Send(ctx, req)
		return redis.AsError(result)
	}, s.DefaultOpts...)
	return
}

// SendMany wraps s.Sync.SendMany in the retry logic.
func (s RetrySync) SendMany(ctx context.Context, reqs []redis.Request) (results []interface{}) {
	_ = retrybp.Do(ctx, func() error {
		results = s.Sync.SendMany(ctx, reqs)
		var err error
		if first := redis.AsError(results[0]); first != nil {
			reqFailed := true
			for i := 1; i < len(results) && reqFailed; i++ {
				asErr := redis.AsError(results[i])
				if !errors.Is(asErr, first) {
					reqFailed = false
				}
			}
			if reqFailed {
				err = first
			}
		}
		return err
	}, s.DefaultOpts...)
	return
}

// SendTransaction wraps s.Sync.SendTransaction in the retry logic.
func (s RetrySync) SendTransaction(ctx context.Context, reqs []redis.Request) (results []interface{}, err error) {
	err = retrybp.Do(ctx, func() error {
		var rErr error
		results, rErr = s.Sync.SendTransaction(ctx, reqs)
		return rErr
	}, s.DefaultOpts...)
	return
}

// Scanner returns a new RetryScanIterator with the same Options as s.
func (s RetrySync) Scanner(ctx context.Context, opts redis.ScanOpts) redisx.ScanIterator {
	retryOpts := make([]retry.Option, 0, len(s.DefaultOpts))
	retryOpts = append(retryOpts, s.DefaultOpts...)
	return RetryScanIterator{
		ScanIterator: s.Sync.Scanner(ctx, opts),
		ctx:          ctx,
		opts:         retryOpts,
	}
}

// RetryScanIterator is a ScanIterator that is wrapped with retry logic.
type RetryScanIterator struct {
	redisx.ScanIterator

	ctx  context.Context
	opts []retry.Option
}

// Next wraps s.ScanIterator.Next in the retry logic.
func (s RetryScanIterator) Next() (results []string, err error) {
	err = retrybp.Do(s.ctx, func() error {
		var nextErr error
		results, nextErr = s.ScanIterator.Next()
		return nextErr
	}, s.opts...)
	return
}
