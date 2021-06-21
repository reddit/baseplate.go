package redisbp_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/reddit/baseplate.go/mqsend"
	"github.com/reddit/baseplate.go/redis/deprecated/redisbp"
	"github.com/reddit/baseplate.go/tracing"
)

const (
	testTimeout = time.Millisecond * 100
)

func TestNewMonitoredClient(t *testing.T) {
	defer func() {
		tracing.CloseTracer()
		tracing.InitGlobalTracer(tracing.TracerConfig{})
	}()
	mmq := mqsend.OpenMockMessageQueue(mqsend.MessageQueueConfig{
		MaxQueueSize:   100,
		MaxMessageSize: 1024,
	})
	logger, startFailing := tracing.TestWrapper(t)
	if err := tracing.InitGlobalTracer(tracing.TracerConfig{
		SampleRate:               1,
		MaxRecordTimeout:         testTimeout,
		Logger:                   logger,
		TestOnlyMockMessageQueue: mmq,
	}); err != nil {
		t.Fatal(err)
	}

	startFailing()

	s, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	client := redisbp.NewMonitoredClient("redis", &redis.Options{Addr: s.Addr()})
	ctx := context.Background()
	if resp := client.Ping(ctx); resp.Err() != nil {
		t.Fatal(resp.Err())
	}

	ctx, cancel := context.WithTimeout(ctx, testTimeout)
	defer cancel()
	msg, err := mmq.Receive(ctx)
	if err != nil {
		t.Fatal(err)
	}

	var trace tracing.ZipkinSpan
	err = json.Unmarshal(msg, &trace)
	if err != nil {
		t.Fatal(err)
	}
	if len(trace.BinaryAnnotations) == 0 {
		t.Error("no binary annotations")
	}

	if err := client.Close(); err != nil {
		t.Fatal(err)
	}

	if resp := client.Ping(ctx); resp.Err() == nil {
		t.Fatal("expected an error, got nil")
	}
}
