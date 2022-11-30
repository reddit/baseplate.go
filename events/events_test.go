package events

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/reddit/baseplate.go/mqsend"

	"github.com/apache/thrift/lib/go/thrift"
)

type mockTStruct struct{}

func (mockTStruct) Read(_ context.Context, _ thrift.TProtocol) error {
	return nil
}

func (mockTStruct) Write(ctx context.Context, p thrift.TProtocol) error {
	if err := p.WriteMessageBegin(ctx, "mock", thrift.CALL, 0); err != nil {
		return err
	}
	return p.WriteMessageEnd(ctx)
}

func TestV2(t *testing.T) {
	const queueSize = 100
	const timeout = time.Millisecond * 10
	const doubleTime = timeout * 2
	const tripleTime = timeout * 3

	// init
	queue := mqsend.OpenMockMessageQueue(mqsend.MessageQueueConfig{
		MaxMessageSize: 1024,
		MaxQueueSize:   queueSize,
	})
	v2 := v2WithConfig(
		Config{
			MaxPutTimeout: timeout,
		},
		queue,
	)

	// put
	var wg sync.WaitGroup
	var failed atomic.Int64
	const expectedFailures = 1
	const n = queueSize + expectedFailures

	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), doubleTime)
			defer cancel()
			before := time.Now()
			if err := v2.Put(ctx, mockTStruct{}); err != nil {
				t.Log("Put failed with:", err)
				failed.Add(1)
			}
			elapsed := time.Since(before)
			if elapsed > tripleTime {
				t.Errorf(
					"Expected timeout at %v, actual elapsed time is %v",
					timeout,
					elapsed,
				)
			}
		}()
	}
	wg.Wait()

	actualFailures := failed.Load()
	if actualFailures != expectedFailures {
		t.Errorf(
			"Expected %d failed Put call, actual %d",
			expectedFailures,
			actualFailures,
		)
	}

	// verify put data
	const expected = "[1,\"mock\",1,0]"
	for i := 0; i < queueSize; i++ {
		func() {
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			data, err := queue.Receive(ctx)
			if err != nil {
				t.Error(err)
			} else {
				if string(data) != expected {
					t.Errorf("data expected to be %q, got %q", expected, data)
				}
			}
		}()
	}

	// PutRaw
	failed.Store(0)
	const rawData = "hello, world"
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), doubleTime)
			defer cancel()
			before := time.Now()
			if err := v2.PutRaw(ctx, []byte(rawData)); err != nil {
				t.Log("PutRaw failed with:", err)
				failed.Add(1)
			}
			elapsed := time.Since(before)
			if elapsed > tripleTime {
				t.Errorf(
					"Expected timeout at %v, actual elapsed time is %v",
					timeout,
					elapsed,
				)
			}
		}()
	}
	wg.Wait()

	actualFailures = failed.Load()
	if actualFailures != expectedFailures {
		t.Errorf(
			"Expected %d failed Put call, actual %d",
			expectedFailures,
			actualFailures,
		)
	}

	// verify PutRaw data
	for i := 0; i < queueSize; i++ {
		func() {
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			data, err := queue.Receive(ctx)
			if err != nil {
				t.Error(err)
			} else {
				if string(data) != rawData {
					t.Errorf("data expected to be %q, got %q", rawData, data)
				}
			}
		}()
	}
}
