package redisxtest

import (
	"context"
	"testing"

	"github.com/joomcode/redispipe/redis"
	"github.com/joomcode/redispipe/redisconn"

	"github.com/reddit/baseplate.go/redis/cache/redisx"
)

// NewMockRedisClient sets up a redis client for use in testing
func NewMockRedisClient(
	ctx context.Context,
	tb testing.TB,
	address string,
	opts redisconn.Opts,
) (*redisx.BaseSync, error) {
	tb.Helper()

	// Create connection to redis
	conn, err := redisconn.Connect(ctx, address, opts)
	if err != nil {
		return nil, err
	}

	// Create client
	client := redisx.BaseSync{
		SyncCtx: redis.SyncCtx{S: conn},
	}

	// Cleanup
	tb.Cleanup(func() {
		conn.Close()
	})

	return &client, nil
}
