package thrifttest_test

import (
	"context"
	"errors"
	"testing"

	"github.com/reddit/baseplate.go/clientpool"
	"github.com/reddit/baseplate.go/thriftbp/thrifttest"
)

func TestMockClientPool(t *testing.T) {
	pool := thrifttest.MockClientPool{}

	t.Run(
		"default",
		func(t *testing.T) {
			if pool.IsExhausted() {
				t.Error("Expected default MockClientPool to report not exhausted.")
			}

			if _, err := pool.Call(context.Background(), "test", nil, nil); err != nil {
				t.Fatal(err)
			}
		},
	)

	pool.Exhausted = true

	t.Run(
		"exhausted",
		func(t *testing.T) {
			if !pool.IsExhausted() {
				t.Error("Expected MockClientPool to report exhausted when set to true")
			}

			_, err := pool.Call(context.Background(), "test", nil, nil)
			if !errors.Is(err, clientpool.ErrExhausted) {
				t.Errorf("Expected returned error to wrap exhausted error, got %v", err)
			}
		},
	)
}
