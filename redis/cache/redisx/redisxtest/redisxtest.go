package redisxtest

import (
	"context"

	"github.com/alicebob/miniredis/v2"
	"github.com/joomcode/redispipe/redis"
	"github.com/joomcode/redispipe/redisconn"

	"github.com/reddit/baseplate.go/redis/cache/redisx"
)

// MockRedisCluster wraps a local version of redis
type MockRedisCluster struct {
	redisCluster *miniredis.Miniredis
}

func NewMockRedisCluster() (mockRedisCluster MockRedisCluster, err error) {
	redisCluster, err := miniredis.Run()
	if err != nil {
		return MockRedisCluster{}, err
	}

	return MockRedisCluster{
		redisCluster: redisCluster,
	}, nil
}

// Addr returns address of mock redis cluster e.g. '127.0.0.1:12345'.
func (mrc *MockRedisCluster) Addr() string {
	return mrc.redisCluster.Addr()
}

// Close shuts down the MockRedisCluster
func (mrc *MockRedisCluster) Close() error {
	mrc.redisCluster.Close()
	return nil
}

// NewMockRedisClient sets up a client and sender to a mock redis cluster
func NewMockRedisClient(
	ctx context.Context,
	address string,
	opts redisconn.Opts,
) (client redisx.BaseSync, teardown func(), err error) {

	// Create connection
	conn, err := redisconn.Connect(ctx, address, opts)
	if err != nil {
		return redisx.BaseSync{}, nil, err
	}

	// Create client
	client = redisx.BaseSync{
		SyncCtx: redis.SyncCtx{S: conn},
	}

	// Teardown closure
	teardown = func() {
		conn.Close()
	}

	return client, teardown, nil
}
