package redisbp_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis"
	"github.com/go-redis/redis/v7"
	"github.com/reddit/baseplate.go/mqsend"
	"github.com/reddit/baseplate.go/redisbp"
	"github.com/reddit/baseplate.go/tracing"
)

const (
	testTimeout = time.Millisecond * 100
)

func TestMonitoredCmdableFactory(t *testing.T) {
	defer func() {
		tracing.CloseTracer()
		tracing.InitGlobalTracer(tracing.TracerConfig{})
	}()
	mmq := mqsend.OpenMockMessageQueue(mqsend.MessageQueueConfig{
		MaxQueueSize:   100,
		MaxMessageSize: 1024,
	})
	logger, startFailing := tracing.TestWrapper(t)
	tracing.InitGlobalTracer(tracing.TracerConfig{
		SampleRate:               1,
		MaxRecordTimeout:         testTimeout,
		Logger:                   logger,
		TestOnlyMockMessageQueue: mmq,
	})
	startFailing()

	s, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	factory := redisbp.NewMonitoredClientFactory(
		"redis",
		redis.NewClient(&redis.Options{Addr: s.Addr()}),
	)
	client := factory.BuildClient(context.Background())
	if resp := client.Ping(); resp.Err() != nil {
		t.Fatal(resp.Err())
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
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

	if err := factory.Close(); err != nil {
		t.Fatal(err)
	}

	// verify that closing the factory closes out the connection pools for the
	// clients it created.
	if resp := client.Ping(); resp.Err() == nil {
		t.Fatal("expected an error, got nil")
	}
}
