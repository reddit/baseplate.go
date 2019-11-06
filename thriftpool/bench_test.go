package thriftpool_test

import (
	"testing"

	"github.com/reddit/baseplate.go/thriftpool"
)

func BenchmarkPoolGetRelease(b *testing.B) {
	opener := func() (thriftpool.Client, error) {
		return &testClient{}, nil
	}

	const min, max = 0, 100
	channelPool, _ := thriftpool.NewChannelPool(min, max, opener)

	for label, pool := range map[string]thriftpool.Pool{
		"channel": channelPool,
	} {
		b.Run(
			label,
			func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					c, err := pool.Get()
					if err != nil {
						b.Fatalf("pool.Get returned error on run #%d: %v", i, err)
					}

					if err := pool.Release(c); err != nil {
						b.Fatalf("pool.Release returned error on run #%d: %v", i, err)
					}
				}
			},
		)
	}
}
