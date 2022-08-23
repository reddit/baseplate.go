package redisbp

import (
	"sync"
	"testing"

	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"

	//lint:ignore SA1019 This library is internal only, not actually deprecated
	"github.com/reddit/baseplate.go/internalv2compat"
)

type fakeClient redis.Client

// PoolStats returns connection pool stats.
func (c fakeClient) PoolStats() *redis.PoolStats {
	return &redis.PoolStats{
		Hits:     1,
		Misses:   2,
		Timeouts: 3,

		TotalConns: 4,
		IdleConns:  5,
		StaleConns: 6,
	}
}

func TestRedisPoolExporterRegister(t *testing.T) {
	exporters := []exporter{
		{
			client: fakeClient{},
			name:   "foo",
		},
		{
			client: fakeClient{},
			name:   "bar",
		},
	}
	for i, exporter := range exporters {
		if err := internalv2compat.GlobalRegistry.Register(exporter); err != nil {
			t.Errorf("Register #%d failed: %v", i, err)
		}
	}
}

func TestRedisPoolExporterCollect(t *testing.T) {
	exporter := exporter{
		client: fakeClient{},
		name:   "test",
	}
	ch := make(chan prometheus.Metric)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Drain the channel
		for range ch {
		}
	}()
	t.Cleanup(func() {
		close(ch)
		wg.Wait()
	})

	// No real test here, we just want to make sure that Collect call will not
	// panic, which would happen if we have a label mismatch.
	exporter.Collect(ch)
}
