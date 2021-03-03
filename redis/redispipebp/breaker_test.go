package redispipebp_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/joomcode/redispipe/redis"
	"github.com/reddit/baseplate.go/breakerbp"
	"github.com/sony/gobreaker"

	"github.com/reddit/baseplate.go/redis/redispipebp"
	"github.com/reddit/baseplate.go/redis/redisx"
)

var errExampleSync = errors.New("example sync error")

type alwaysError struct{}

func (a alwaysError) Do(context.Context, string, ...interface{}) interface{} {
	return errExampleSync
}

func (a alwaysError) Send(context.Context, redis.Request) interface{} {
	return errExampleSync
}

func (a alwaysError) SendMany(_ context.Context, reqs []redis.Request) []interface{} {
	results := make([]interface{}, 0, len(reqs))
	for range reqs {
		results = append(results, errExampleSync)
	}
	return results
}

func (a alwaysError) SendTransaction(_ context.Context, reqs []redis.Request) ([]interface{}, error) {
	return nil, errExampleSync
}

func (a alwaysError) Scanner(context.Context, redis.ScanOpts) redisx.ScanIterator {
	return alwaysErrorScanner{}
}

type alwaysErrorScanner struct{}

func (a alwaysErrorScanner) Next() ([]string, error) {
	return []string{}, errExampleSync
}

func checkForBreakerError(t *testing.T, resp interface{}) {
	t.Helper()

	err := redis.AsError(resp)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !errors.Is(err, gobreaker.ErrOpenState) {
		t.Fatalf("expected a %+v error, got %+v", gobreaker.ErrOpenState, err)
	}
}

func TestBreakerSync(t *testing.T) {
	defer flushRedis()

	ctx := context.Background()
	bClient := redispipebp.BreakerSync{
		Sync: alwaysError{},
		Breaker: breakerbp.NewFailureRatioBreaker(breakerbp.Config{
			MinRequestsToTrip: 0,
			FailureThreshold:  0.001,
			Name:              "test-breaker",
			Timeout:           time.Hour,
		}),
	}

	// Make an initial call to trip the circuit breaker
	if err := redis.AsError(bClient.Do(ctx, "PING")); err == nil {
		t.Fatal("expected an error, got nil")
	}

	t.Run("Do", func(t *testing.T) {
		checkForBreakerError(t, redis.AsError(bClient.Do(ctx, "PING")))
	})

	t.Run("Send", func(t *testing.T) {
		checkForBreakerError(t, bClient.Send(ctx, redis.Req("PING")))
	})

	t.Run("SendMany", func(t *testing.T) {
		res := bClient.SendMany(ctx, []redis.Request{
			redis.Req("PING"),
			redis.Req("SET", "key", "value"),
		})
		for _, r := range res {
			checkForBreakerError(t, r)
		}
	})

	t.Run("SendTransaction", func(t *testing.T) {
		_, err := bClient.SendTransaction(ctx, []redis.Request{
			redis.Req("PING"),
			redis.Req("SET", "KEY", "VALUE"),
		})
		checkForBreakerError(t, err)
	})

	t.Run("Scanner", func(t *testing.T) {
		s := bClient.Scanner(ctx, redis.ScanOpts{
			Cmd:   "KEYS",
			Match: "*",
		})
		_, err := s.Next()
		checkForBreakerError(t, err)
	})
}
