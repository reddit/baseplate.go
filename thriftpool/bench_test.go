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
				b.RunParallel(
					func(pb *testing.PB) {
						for pb.Next() {
							c, err := pool.Get()
							if err != nil {
								b.Fatalf("pool.Get returned error: %v", err)
							}

							if err := pool.Release(c); err != nil {
								b.Fatalf("pool.Release returned error: %v", err)
							}
						}
					},
				)
			},
		)
	}
}
