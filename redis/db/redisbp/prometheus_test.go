package redisbp

import (
	"sync"
	"testing"

	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"
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

func TestRedisPoolExporter(t *testing.T) {
	client := &fakeClient{}

	exporter := newExporter(
		client,
		"test",
	)
	// No real test here, we just want to make sure that Collect call will not
	// panic, which would happen if we have a label mismatch.
	ch := make(chan prometheus.Metric)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range ch {
		}
	}()
	exporter.Collect(ch)
	close(ch)
	wg.Wait()
}
