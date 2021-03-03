package redispipebp

import (
	"context"
	"errors"

	"github.com/joomcode/errorx"
	"github.com/joomcode/redispipe/redis"

	"github.com/reddit/baseplate.go/redis/redisx"
)

// RedispipeError wraps an errorx.Err from redispipe and provides an interface that
// interacts with the errors package provided by Go.
type RedispipeError struct {
	// Errorx is the Error wrapped by the RedispipeError.
	// Note, this is not embedded because Error ends up conflicting with the
	// Error() function we must implement to fufill the error interface.
	Errorx *errorx.Error
}

// As implements helper interface for errors.As.
func (e *RedispipeError) As(v interface{}) bool {
	if target, ok := v.(**RedispipeError); ok {
		*target = e
		return true
	}
	if target, ok := v.(**errorx.Error); ok {
		*target = e.Errorx
		return true
	}
	return errors.As(e.Unwrap(), v)
}

// Unwrap implements helper interface for errors.Unwrap.
//
// Unwraps the underlying errorx.Error by calling Cause() and wrapping
// the result.
func (e *RedispipeError) Unwrap() error {
	return wrapRedispipeError(e.Errorx.Cause())
}

func (e *RedispipeError) Error() string {
	return e.Errorx.Error()
}

var (
	_ error = (*RedispipeError)(nil)
)

func wrapRedispipeError(err error) error {
	var e *errorx.Error
	if errors.As(err, &e) {
		return &RedispipeError{Errorx: e}
	}
	return err
}

// WrapErrorsSync takes any errors returned by redispipe and wraps them in a new
// type that is compatible with the built-in errors package.
type WrapErrorsSync struct {
	Sync redisx.Sync
}

// Do calls s.Sync.Do and if it returns an error, wraps the error in a RedispipeError.
func (s WrapErrorsSync) Do(ctx context.Context, cmd string, args ...interface{}) interface{} {
	result := s.Sync.Do(ctx, cmd, args...)
	if err := redis.AsError(result); err != nil {
		return wrapRedispipeError(err)
	}
	return result
}

// Send calls s.Sync.Send and if it returns an error, wraps the error in a RedispipeError.
func (s WrapErrorsSync) Send(ctx context.Context, r redis.Request) interface{} {
	result := s.Sync.Send(ctx, r)
	if err := redis.AsError(result); err != nil {
		return wrapRedispipeError(err)
	}
	return result
}

// SendMany calls s.Sync.SendMany and if it returns any errors, wraps them in a RedispipeError.
func (s WrapErrorsSync) SendMany(ctx context.Context, reqs []redis.Request) []interface{} {
	res := s.Sync.SendMany(ctx, reqs)
	results := make([]interface{}, 0, len(res))
	for _, r := range res {
		if err := redis.AsError(r); err != nil {
			r = wrapRedispipeError(err)
		}
		results = append(results, r)
	}
	return results
}

// SendTransaction calls s.Sync.SendTransaction and if it returns an error, wraps the error in a RedispipeError.
func (s WrapErrorsSync) SendTransaction(ctx context.Context, reqs []redis.Request) ([]interface{}, error) {
	results, err := s.Sync.SendTransaction(ctx, reqs)
	return results, wrapRedispipeError(err)
}

// Scanner returns a new WrapErrorsScanIterator.
func (s WrapErrorsSync) Scanner(ctx context.Context, opts redis.ScanOpts) redisx.ScanIterator {
	return WrappedErrorsScanIterator{s.Sync.Scanner(ctx, opts)}
}

// WrappedErrorsScanIterator takes any errors returned by redispipe and wraps them in a new
// type that is compatible with the built-in errors package.
type WrappedErrorsScanIterator struct {
	redisx.ScanIterator
}

// Next calls s.ScanIterator.Next and if it returns an error, wraps the error in a RedispipeError.
func (s WrappedErrorsScanIterator) Next() ([]string, error) {
	results, err := s.ScanIterator.Next()
	err = wrapRedispipeError(err)
	return results, err
}

var (
	_ redisx.Sync         = WrapErrorsSync{}
	_ redisx.ScanIterator = WrappedErrorsScanIterator{}
)
