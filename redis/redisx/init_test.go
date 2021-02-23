package redisx_test

import (
	"context"
	"os"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/joomcode/redispipe/redis"
	"github.com/joomcode/redispipe/redisconn"

	"github.com/reddit/baseplate.go/redis/redisx"
)

var (
	client redisx.Syncx
)

func TestMain(m *testing.M) {
	s, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	defer s.Close()

	sender, err := redisconn.Connect(context.TODO(), s.Addr(), redisconn.Opts{})
	if err != nil {
		panic(err)
	}
	defer sender.Close()

	client = redisx.Syncx{redisx.BaseSync{
		SyncCtx: redis.SyncCtx{S: sender},
	}}

	flushRedis()
	os.Exit(m.Run())
}

func flushRedis() {
	if resp := client.Do(context.Background(), nil, "FLUSHALL"); redis.AsError(resp) != nil {
		panic(resp)
	}
}
