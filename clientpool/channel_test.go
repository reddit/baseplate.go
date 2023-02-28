package clientpool_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/reddit/baseplate.go/clientpool"
)

func TestChannelPoolInvalidConfig(t *testing.T) {
	const min, init, max = 5, 1, 1
	_, err := clientpool.NewChannelPool(context.Background(), min, init, max, nil)
	if err == nil {
		t.Errorf(
			"NewChannelPool with min %d and max %d expected an error, got nil.",
			min,
			max,
		)
	}
}

func TestChannelPool(t *testing.T) {
	opener := func(called *atomic.Int32) clientpool.ClientOpener {
		return func() (clientpool.Client, error) {
			if called != nil {
				called.Add(1)
			}
			return &testClient{}, nil
		}
	}

	const min, init, max = 1, 2, 5
	var openerCalled atomic.Int32
	pool, err := clientpool.NewChannelPool(context.Background(), min, init, max, opener(&openerCalled))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("min: %d, init: %d, max: %d", min, init, max)

	testPool(t, pool, &openerCalled, init, max)
}

func TestChannelPoolWithOpenerFailure(t *testing.T) {
	// In this opener, every other call will fail
	opener := func() clientpool.ClientOpener {
		var called atomic.Int32
		failure := errors.New("failed")
		return func() (clientpool.Client, error) {
			if called.Add(1)%2 == 0 {
				return nil, failure
			}
			return &testClient{}, nil
		}
	}

	const min, init, max = 0, 2, 5
	t.Run(
		"new-with-init-2-should-not-fail-initialization",
		func(t *testing.T) {
			pool, err := clientpool.NewChannelPool(context.Background(), min, init, max, opener())
			if err != nil {
				t.Errorf(
					"NewChannelPool with (min, init, max) = (%d, %d, %d) failed with: %v",
					min,
					init,
					max,
					err,
				)
			}
			if pool == nil {
				t.Errorf(
					"NewChannelPool with (min, init, max) = (%d, %d, %d) should return non-nil pool",
					min,
					init,
					max,
				)
			}
		},
	)

	pool, err := clientpool.NewChannelPool(context.Background(), min, init, max, opener())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("min: %d, max: %d", min, max)

	t.Run(
		"drain-the-pool",
		func(t *testing.T) {
			for i := 0; i < init; i++ {
				_, err := pool.Get()
				if err != nil {
					t.Errorf("pool.Get returned error: %v", err)
				}
			}

			checkActiveAndAllocated(t, pool, init, 0)
		},
	)

	t.Run(
		"get-one-more-with-failed-opener",
		func(t *testing.T) {
			// The next opener call would fail
			_, err := pool.Get()
			if err == nil {
				t.Error("pool.Get should return error, got nil")
			}

			checkActiveAndAllocated(t, pool, init, 0)
		},
	)
}
