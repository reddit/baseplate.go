package redisxtest

import (
	"context"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/joomcode/redispipe/redis"
	"github.com/joomcode/redispipe/redisconn"

	"github.com/reddit/baseplate.go/redis/cache/redisx"
)

// MockRedisCluster wraps a local version of redis
type MockRedisCluster struct {
	redisCluster *miniredis.Miniredis
	teardown     func()
}

func NewMockRedisCluster() (MockRedisCluster, error) {
	redisCluster, err := miniredis.Run()
	if err != nil {
		return MockRedisCluster{}, err
	}

	teardown := func() {
		redisCluster.Close()
	}

	return MockRedisCluster{
		redisCluster: redisCluster,
		teardown:     teardown,
	}, nil
}

// Addr returns address of mock redis cluster e.g. '127.0.0.1:12345'.
func (mrc *MockRedisCluster) Addr() string {
	return mrc.redisCluster.Addr()
}

// Close shuts down the MockRedisCluster
func (mrc *MockRedisCluster) Close() {
	mrc.redisCluster.Close()
}

// NewMockRedisClient sets up a client and sender to a mock redis cluster
func NewMockRedisClient(ctx context.Context, redisCluster MockRedisCluster, timeout time.Duration) (client redisx.Syncx, teardown func(), err error) {

	// Create connection
	conn, err := redisconn.Connect(ctx, redisCluster.Addr(), redisconn.Opts{IOTimeout: timeout})
	if err != nil {
		redisCluster.Close()
		return redisx.Syncx{}, nil, err
	}

	// Create client
	client = redisx.Syncx{
		Sync: redisx.BaseSync{SyncCtx: redis.SyncCtx{S: conn}},
	}

	// Teardown closure
	teardown = func() {
		conn.Close()
	}

	return client, teardown, nil
}
