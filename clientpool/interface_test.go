package clientpool_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/reddit/baseplate.go/clientpool"
)

type testClient struct {
	closed bool
}

func (tc *testClient) IsOpen() bool {
	return !tc.closed
}

func (tc *testClient) Close() error {
	tc.closed = true
	return nil
}

func checkActiveAndAllocated(t *testing.T, pool clientpool.Pool, expectedActive, expectedAllocated int) {
	t.Helper()

	active := pool.NumActiveClients()
	if active != int32(expectedActive) {
		t.Errorf(
			"pool.NumActiveClients() expected %d, got %d",
			expectedActive,
			active,
		)
	}

	allocated := pool.NumAllocated()
	if allocated != int32(expectedAllocated) {
		t.Errorf(
			"pool.NumAllocated() expected %d, got %d",
			expectedAllocated,
			allocated,
		)
	}
}

func testPool(t *testing.T, pool clientpool.Pool, openerCalled *atomic.Int32, min, max int) {
	t.Run(
		"drain-the-pool",
		func(t *testing.T) {
			for i := 0; i < min; i++ {
				_, err := pool.Get()
				if err != nil {
					t.Errorf("pool.Get returned error: %v", err)
				}
			}

			checkActiveAndAllocated(t, pool, min, 0)

			t.Logf("opener called %d times", openerCalled.Load())
		},
	)

	t.Run(
		"get-one-more",
		func(t *testing.T) {
			_, err := pool.Get()
			if err != nil {
				t.Errorf("pool.Get returned error: %v", err)
			}

			checkActiveAndAllocated(t, pool, min+1, 0)

			t.Logf("opener called %d times", openerCalled.Load())
		},
	)

	t.Run(
		"get-to-max",
		func(t *testing.T) {
			for i := 0; i < max-min-1; i++ {
				_, err := pool.Get()
				if err != nil {
					t.Errorf("pool.Get returned error: %v", err)
				}
			}

			checkActiveAndAllocated(t, pool, max, 0)

			t.Logf("opener called %d times", openerCalled.Load())
		},
	)

	t.Run(
		"get-one-more-after-max",
		func(t *testing.T) {
			exhausted := pool.IsExhausted()
			if !exhausted {
				t.Errorf(
					"pool.IsExhausted() expected true, got false. Allocated = %d, Active = %d",
					pool.NumAllocated(),
					pool.NumActiveClients(),
				)
			}

			beforeOpenerCalled := openerCalled.Load()
			_, err := pool.Get()
			if err == nil {
				t.Error("pool.Get expected error, got nil")
			}

			diff := openerCalled.Load() - beforeOpenerCalled
			if diff != 0 {
				t.Errorf("pool.Get should not call opener, called %d times", diff)
			}

			checkActiveAndAllocated(t, pool, max, 0)

			t.Logf("opener called %d times", openerCalled.Load())
		},
	)

	t.Run(
		"get-should-not-return-closed-client",
		func(t *testing.T) {
			// This test relies on the pool being empty
			if n := pool.NumAllocated(); n != 0 {
				t.Fatalf("The pool should be empty, but has %d allocated instead", n)
			}

			c := &testClient{}
			if err := pool.Release(c); err != nil {
				t.Errorf("pool.Release returned error: %v", err)
			}
			// Close the client in the pool
			c.closed = true

			beforeOpenerCalled := openerCalled.Load()
			newc, err := pool.Get()
			if err != nil {
				t.Fatalf("pool.Get returned error: %v", err)
			}
			if !newc.IsOpen() {
				t.Error("pool.Get returned closed client")
			}
			diff := openerCalled.Load() - beforeOpenerCalled
			if diff != 1 {
				t.Error("opener not called with closed client")
			}

			t.Logf("opener called %d times", openerCalled.Load())
		},
	)

	t.Run(
		"release-min-closed-clients",
		func(t *testing.T) {
			beforeOpenerCalled := openerCalled.Load()

			for i := 0; i < min; i++ {
				c := &testClient{
					closed: true,
				}
				if err := pool.Release(c); err != nil {
					t.Errorf("pool.Release returned error: %v", err)
				}
			}

			diff := openerCalled.Load() - beforeOpenerCalled
			if int(diff) != min {
				t.Errorf(
					"Expected opener to be called %d times, called %d times instead",
					min,
					diff,
				)
			}

			checkActiveAndAllocated(t, pool, max-min, min)

			t.Logf("opener called %d times", openerCalled.Load())
		},
	)

	t.Run(
		"release-to-max-minus-1",
		func(t *testing.T) {
			beforeOpenerCalled := openerCalled.Load()

			for i := 0; i < max-min-1; i++ {
				c := &testClient{}
				if err := pool.Release(c); err != nil {
					t.Errorf("pool.Release returned error: %v", err)
				}
			}

			diff := openerCalled.Load() - beforeOpenerCalled
			if diff != 0 {
				t.Errorf(
					"Didn't expect opener to be called, called %d times instead",
					diff,
				)
			}

			checkActiveAndAllocated(t, pool, 1, max-1)

			t.Logf("opener called %d times", openerCalled.Load())
		},
	)

	lastClient := &testClient{}

	t.Run(
		"release-to-max",
		func(t *testing.T) {
			if err := pool.Release(lastClient); err != nil {
				t.Errorf("pool.Release returned error: %v", err)
			}

			if lastClient.closed {
				t.Error("pool.Release should not close client released")
			}

			checkActiveAndAllocated(t, pool, 0, max)

			t.Logf("opener called %d times", openerCalled.Load())
		},
	)

	t.Run(
		"release-one-more",
		func(t *testing.T) {
			c := &testClient{}
			if err := pool.Release(c); err != nil {
				t.Errorf("pool.Release returned error: %v", err)
			}

			checkActiveAndAllocated(t, pool, -1, max)

			if !c.closed {
				t.Error("pool.Release did not close extra released client")
			}

			t.Logf("opener called %d times", openerCalled.Load())
		},
	)

	t.Run(
		"concurrency",
		func(t *testing.T) {
			n := max * 2
			var wg sync.WaitGroup
			wg.Add(n)
			begin := time.Now()
			for i := 0; i < n; i++ {
				go func(i int) {
					defer wg.Done()
					client, err := pool.Get()
					if err != nil {
						t.Errorf("pool.Get on #%d failed with: %v", i, err)
					} else {
						if err := pool.Release(client); err != nil {
							t.Errorf("pool.Release on #%d failed with: %v", i, err)
						}
					}
				}(i)
			}
			wg.Wait()
			t.Logf("%d Get/Release took %v", n, time.Since(begin))
		},
	)

	t.Run(
		"close-pool",
		func(t *testing.T) {
			if err := pool.Close(); err != nil {
				t.Errorf("pool.Close returned error: %v", err)
			}

			if !lastClient.closed {
				t.Error("pool.Close did not close client")
			}

			t.Logf("opener called %d times", openerCalled.Load())
		},
	)
}
