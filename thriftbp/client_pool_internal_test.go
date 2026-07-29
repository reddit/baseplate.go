package thriftbp

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/reddit/baseplate.go/clientpool"
)

type cancellationTestClient struct {
	open atomic.Bool

	callStarted   chan struct{}
	callReturned  chan struct{}
	callUnblocked chan struct{}
	closeStarted  chan struct{}
	allowClose    chan struct{}

	closeCalls atomic.Int32
}

func newCancellationTestClient() *cancellationTestClient {
	c := &cancellationTestClient{
		callStarted:   make(chan struct{}),
		callReturned:  make(chan struct{}),
		callUnblocked: make(chan struct{}),
		closeStarted:  make(chan struct{}),
		allowClose:    make(chan struct{}),
	}
	c.open.Store(true)
	return c
}

func (c *cancellationTestClient) Call(
	context.Context,
	string,
	thrift.TStruct,
	thrift.TStruct,
) (thrift.ResponseMeta, error) {
	close(c.callStarted)
	<-c.callUnblocked
	close(c.callReturned)
	return thrift.ResponseMeta{}, errors.New("connection closed")
}

func (c *cancellationTestClient) Close() error {
	c.closeCalls.Add(1)
	c.open.Store(false)
	close(c.closeStarted)
	close(c.callUnblocked)
	<-c.allowClose
	return nil
}

func (c *cancellationTestClient) IsOpen() bool {
	return c.open.Load()
}

type cancellationTestPool struct {
	client    Client
	released  chan bool
	discarded chan bool
}

func (p *cancellationTestPool) Get() (clientpool.Client, error) {
	return p.client, nil
}

func (p *cancellationTestPool) Release(c clientpool.Client) error {
	p.released <- c.IsOpen()
	return nil
}

func (p *cancellationTestPool) Discard(c clientpool.Client) error {
	p.discarded <- c.IsOpen()
	return nil
}

func (*cancellationTestPool) Close() error {
	return nil
}

func (*cancellationTestPool) NumActiveClients() int32 {
	return 0
}

func (*cancellationTestPool) NumAllocated() int32 {
	return 1
}

func (*cancellationTestPool) IsExhausted() bool {
	return false
}

func TestPooledCallCancellationClosesAndDiscardsClient(t *testing.T) {
	client := newCancellationTestClient()
	pool := &cancellationTestPool{
		client:    client,
		released:  make(chan bool, 1),
		discarded: make(chan bool, 1),
	}
	p := &clientPool{
		Pool: pool,
		slug: "cancellation-test",
	}

	ctx, cancel := context.WithCancelCause(context.Background())
	cause := errors.New("test cancellation cause")
	result := make(chan error, 1)
	go func() {
		_, err := p.pooledCall(ctx, "method", nil, nil)
		result <- err
	}()

	<-client.callStarted
	cancel(cause)
	<-client.closeStarted
	<-client.callReturned

	select {
	case <-pool.released:
		t.Fatal("client was released before the cancellation callback finished")
	case err := <-result:
		t.Fatalf("pooledCall returned before the cancellation callback finished: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	close(client.allowClose)

	select {
	case err := <-result:
		if !errors.Is(err, cause) {
			t.Fatalf("pooledCall error = %v, want cancellation cause %v", err, cause)
		}
	case <-time.After(time.Second):
		t.Fatal("pooledCall did not return after the cancellation callback finished")
	}

	select {
	case open := <-pool.discarded:
		if open {
			t.Error("closed client was released as reusable")
		}
	default:
		t.Fatal("client was not discarded")
	}

	select {
	case <-pool.released:
		t.Fatal("canceled client was released instead of discarded")
	default:
	}

	if got := client.closeCalls.Load(); got != 1 {
		t.Errorf("client Close calls = %d, want 1", got)
	}
}
