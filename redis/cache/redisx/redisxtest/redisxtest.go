package redisxtest

import (
	"context"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/joomcode/redispipe/redis"
	"github.com/joomcode/redispipe/redisconn"

	"github.com/reddit/baseplate.go/redis/cache/redisx"
)

// NewMockRedisClient sets up a mock redis cluster, client and sender
// This should be called from TestMain since the miniredis instance created is locked
func NewMockRedisClient(ctx context.Context, timeout time.Duration) (client redisx.Syncx, teardown func(), err error) {
	redisCluster, err := miniredis.Run()
	if err != nil {
		return redisx.Syncx{}, nil, err
	}

	conn, err := redisconn.Connect(ctx, redisCluster.Addr(), redisconn.Opts{IOTimeout: timeout})
	if err != nil {
		redisCluster.Close()
		return redisx.Syncx{}, nil, err
	}

	client = redisx.Syncx{
		Sync: redisx.BaseSync{SyncCtx: redis.SyncCtx{S: conn}},
	}

	// Teardown closure
	teardown = func() {
		redisCluster.Close()
		conn.Close()
	}

	return client, teardown, nil
}

func FlushRedis(ctx context.Context, client redisx.Syncx) error {
	return client.Send(ctx, redisx.Req(nil, "FLUSHALL"))
}
