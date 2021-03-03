package redispipebp_test

import (
	"context"
	"errors"
	"testing"

	"github.com/joomcode/errorx"
	"github.com/joomcode/redispipe/redis"

	"github.com/reddit/baseplate.go/redis/redispipebp"
)

func TestRedispipeError(t *testing.T) {
	canceled := &redispipebp.RedispipeError{
		Errorx: redis.ErrRequestCancelled.WrapWithNoMessage(context.Canceled),
	}
	wrapped := &redispipebp.RedispipeError{
		Errorx: redis.ErrIO.WrapWithNoMessage(canceled.Errorx),
	}

	t.Run("Unwrap", func(t *testing.T) {
		if errors.Unwrap(canceled) != context.Canceled {
			t.Errorf("expected to errors.Unwrap to return context.Canceled, got %+v", errors.Unwrap(canceled))
		}
		if canceled.Unwrap() != context.Canceled {
			t.Errorf("expected to Unwrap() to return context.Canceled, got %+v", canceled.Unwrap())
		}
	})

	t.Run("Is", func(t *testing.T) {
		if !errors.Is(canceled, context.Canceled) {
			t.Error("expected 'canceled' to be context.Canceled but was not")
		}

		if !errors.Is(wrapped, context.Canceled) {
			t.Error("expected 'wrapped' to be context.Canceled but was not")
		}
	})

	t.Run("As", func(t *testing.T) {
		var w *redispipebp.RedispipeError
		if !errors.As(canceled, &w) {
			t.Error("Was not able to set cancelled 'As' a RedispipeError")
		}

		var ex *errorx.Error
		if !errors.As(canceled, &ex) {
			t.Error("Was not able to set cancelled 'As' an errorx.Error")
		}
	})
}

func TestWrapErrorsSync(t *testing.T) {
	defer flushRedis()

	wClient := redispipebp.WrapErrorsSync{Sync: client}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	checkForWrappedError := func(t *testing.T, resp interface{}) {
		t.Helper()

		var err error
		if err = redis.AsError(resp); err == nil {
			t.Fatal("expected an error, got nil")
		}
		var e *redispipebp.RedispipeError
		if !errors.As(err, &e) {
			t.Fatalf("expected to get a RedispipeError, got %+v", err)
		}
		var ex *errorx.Error
		if !errors.As(err, &ex) {
			t.Errorf("expected to RedispipeError to be wrapping an errorx.Error, got %+v", err)
		}
		if !ex.IsOfType(redis.ErrRequestCancelled) {
			t.Errorf("expected to get a 'redis.ErrRequestCanceled' error, got %+v", err)
		}
		// This test case should be true, but not all of the places where redispipe returns
		// ErrRequestCancelled actually wrap the underlying context.Canceled.
		// if !errors.Is(err, context.Canceled) {
		// 	t.Errorf("expected error to be 'context.Canceled', but was not")
		// }
	}

	t.Run("Do", func(t *testing.T) {
		r := wClient.Do(ctx, "PING")
		checkForWrappedError(t, r)
	})

	t.Run("Send", func(t *testing.T) {
		r := wClient.Send(ctx, redis.Req("PING"))
		checkForWrappedError(t, r)
	})

	t.Run("SendMany", func(t *testing.T) {
		results := wClient.SendMany(ctx, []redis.Request{
			redis.Req("PING"),
			redis.Req("SET", "KEY", "VALUE"),
		})
		for _, r := range results {
			checkForWrappedError(t, r)
		}
	})

	t.Run("SendTransaction", func(t *testing.T) {
		_, err := wClient.SendTransaction(ctx, []redis.Request{
			redis.Req("PING"),
			redis.Req("SET", "KEY", "VALUE"),
		})
		checkForWrappedError(t, err)
	})

	t.Run("Scanner", func(t *testing.T) {
		s := wClient.Scanner(ctx, redis.ScanOpts{
			Cmd:   "KEYS",
			Match: "*",
		})
		_, err := s.Next()
		checkForWrappedError(t, err)
	})
}
