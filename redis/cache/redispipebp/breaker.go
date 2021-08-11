package redispipebp

import (
	"context"
	"errors"

	"github.com/joomcode/redispipe/redis"

	"github.com/reddit/baseplate.go/breakerbp"
	"github.com/reddit/baseplate.go/redis/cache/redisx"
)

// BreakerSync wraps redis calls is the given CircuitBreaker.
type BreakerSync struct {
	Sync    redisx.Sync
	Breaker breakerbp.CircuitBreaker
}

// Do wraps s.Sync.Do in the given CircuitBreaker
func (s BreakerSync) Do(ctx context.Context, cmd string, args ...interface{}) interface{} {
	res, err := s.Breaker.Execute(func() (interface{}, error) {
		result := s.Sync.Do(ctx, cmd, args...)
		if err := redis.AsError(result); err != nil {
			return nil, err
		}
		return result, nil
	})
	if err != nil {
		return err
	}
	return res
}

// Send wraps s.Sync.Send in the given CircuitBreaker
func (s BreakerSync) Send(ctx context.Context, r redis.Request) interface{} {
	res, err := s.Breaker.Execute(func() (interface{}, error) {
		result := s.Sync.Send(ctx, r)
		if err := redis.AsError(result); err != nil {
			return nil, err
		}
		return result, nil
	})
	if err != nil {
		return err
	}
	return res
}

// SendMany wraps s.Sync.SendMany in the given CircuitBreaker
func (s BreakerSync) SendMany(ctx context.Context, reqs []redis.Request) []interface{} {
	res, err := s.Breaker.Execute(func() (interface{}, error) {
		results := s.Sync.SendMany(ctx, reqs)
		if len(results) == 0 {
			return results, nil
		}
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
		return results, err
	})
	results, _ := res.([]interface{})
	if err != nil {
		errors := make([]interface{}, 0, len(results))
		for range results {
			errors = append(errors, err)
		}
		return errors
	}
	return results
}

// SendTransaction wraps s.Sync.SendTransaction in the given CircuitBreaker
func (s BreakerSync) SendTransaction(ctx context.Context, reqs []redis.Request) ([]interface{}, error) {
	res, err := s.Breaker.Execute(func() (interface{}, error) {
		return s.Sync.SendTransaction(ctx, reqs)
	})
	results, _ := res.([]interface{})
	return results, err
}

// Scanner returns a new BreakerScanIterator using s.Sync.Scanner and the given CircuitBreaker.
func (s BreakerSync) Scanner(ctx context.Context, opts redis.ScanOpts) redisx.ScanIterator {
	return BreakerScanIterator{ScanIterator: s.Sync.Scanner(ctx, opts), cb: s.Breaker}
}

// BreakerScanIterator is a ScanIterator that is wrapped with a circuit breaker.
type BreakerScanIterator struct {
	redisx.ScanIterator

	cb breakerbp.CircuitBreaker
}

// Next wraps s.ScanIterator.Next in the given CircuitBreaker
func (s BreakerScanIterator) Next() ([]string, error) {
	res, err := s.cb.Execute(func() (interface{}, error) {
		return s.ScanIterator.Next()
	})
	results, _ := res.([]string)
	return results, err
}

var (
	_ redisx.Sync         = BreakerSync{}
	_ redisx.ScanIterator = BreakerScanIterator{}
)
