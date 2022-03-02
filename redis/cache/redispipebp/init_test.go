package redispipebp_test

import (
	"context"
	"os"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/joomcode/redispipe/redis"
	"github.com/joomcode/redispipe/redisconn"

	"github.com/reddit/baseplate.go/mqsend"
	"github.com/reddit/baseplate.go/tracing"

	"github.com/reddit/baseplate.go/redis/cache/redisx"
	"github.com/reddit/baseplate.go/redis/cache/redisx/redisxtest"
)

var (
	client redisx.Sync
	mmq    *mqsend.MockMessageQueue
)

func TestMain(m *testing.M) {
	defer func() {
		_ = tracing.CloseTracer()
		_ = tracing.InitGlobalTracer(tracing.Config{})
	}()
	mmq = mqsend.OpenMockMessageQueue(mqsend.MessageQueueConfig{
		MaxQueueSize:   100,
		MaxMessageSize: 1024,
	})
	if err := tracing.InitGlobalTracer(tracing.Config{
		SampleRate:               1,
		MaxRecordTimeout:         testTimeout,
		Logger:                   nil,
		TestOnlyMockMessageQueue: mmq,
	}); err != nil {
		panic(err)
	}

	s, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	defer s.Close()

	redisCluster, clusterTeardown, err := redisxtest.NewMockRedisCluster()
	if err != nil {
		panic(err)
	}
	defer clusterTeardown()

	sender, err := redisconn.Connect(context.TODO(), s.Addr(), redisconn.Opts{})
	if err != nil {
		panic(err)
	}
	defer sender.Close()

	client = redisx.BaseSync{
		SyncCtx: redis.SyncCtx{S: sender},
	}
	var clientTeardown func()
	client, clientTeardown, err = redisxtest.NewMockRedisClient(context.TODO(), redisCluster, redisconn.Opts{})
	defer clientTeardown()
	flushRedis()
	os.Exit(m.Run())
}

func flushRedis() {
	if resp := client.Do(context.Background(), "FLUSHALL"); redis.AsError(resp) != nil {
		panic(resp)
	}
}
