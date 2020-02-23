package thriftclient_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/reddit/baseplate.go/clientpool"
	"github.com/reddit/baseplate.go/thriftclient"
)

const (
	addr = ":0"
)

func TestSingleAddressGenerator(t *testing.T) {
	t.Parallel()

	gen := thriftclient.SingleAddressGenerator(addr)
	generated, err := gen()
	if err != nil {
		t.Fatal(err)
	}
	if generated != addr {
		t.Fatalf("wrong address, expected %q, got %q", addr, generated)
	}
}

func TestMonitoredTTLClientFactory(t *testing.T) {
	fact := thriftclient.MonitoredTTLClientFactory(time.Millisecond * 3)
	trans := thrift.NewTMemoryBuffer()
	protoFact := thrift.NewTBinaryProtocolFactoryDefault()
	client := fact(trans, protoFact)
	if _, ok := client.(*thriftclient.TTLClient); !ok {
		t.Fatal("wrong type for client")
	}
}

func TestUnmonitoredTTLClientFactory(t *testing.T) {
	fact := thriftclient.UnmonitoredTTLClientFactory(time.Millisecond * 3)
	trans := thrift.NewTMemoryBuffer()
	protoFact := thrift.NewTBinaryProtocolFactoryDefault()
	client := fact(trans, protoFact)
	if _, ok := client.(*thriftclient.TTLClient); !ok {
		t.Fatal("wrong type for client")
	}
}

func TestTTLClientPool(t *testing.T) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	pool, err := thriftclient.NewTTLClientPool(
		time.Minute, thriftclient.ClientPoolConfig{
			Addr:               ln.Addr().String(),
			ServiceSlug:        "test",
			InitialConnections: 1,
			MaxConnections:     5,
			SocketTimeout:      time.Millisecond * 10,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, err = pool.GetClient(); err != nil {
		t.Fatal(err)
	}
}

func TestCustomClientPool(t *testing.T) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	cfg := thriftclient.ClientPoolConfig{
		ServiceSlug:        "test",
		InitialConnections: 1,
		MaxConnections:     5,
		SocketTimeout:      time.Millisecond * 10,
	}
	fact := func(trans thrift.TTransport, _ thrift.TProtocolFactory) thriftclient.Client {
		return thriftclient.NewTTLClient(trans, &thriftclient.MockClient{}, time.Minute)
	}
	pool, err := thriftclient.NewCustomClientPool(
		cfg,
		thriftclient.SingleAddressGenerator(ln.Addr().String()),
		fact,
		thrift.NewTBinaryProtocolFactoryDefault(),
	)
	if err != nil {
		t.Fatal(err)
	}

	c, err := pool.GetClient()
	if err != nil {
		t.Fatal(err)
	}

	if err = c.Call(context.Background(), "test", nil, nil); err != nil {
		t.Fatal(err)
	}
}

func TestMockClientPool(t *testing.T) {
	pool := thriftclient.MockClientPool{}

	t.Run(
		"default",
		func(t *testing.T) {
			if pool.IsExhausted() {
				t.Error("Expected default MockClientPool to report not exhausted.")
			}

			c, err := pool.GetClient()
			if err != nil {
				t.Fatal(err)
			}

			if err = c.Call(context.Background(), "test", nil, nil); err != nil {
				t.Fatal(err)
			}
		},
	)

	pool.Exhausted = true

	t.Run(
		"exhausted",
		func(t *testing.T) {
			if !pool.IsExhausted() {
				t.Error("Expected MockClientPool to report exhausted when set to true")
			}

			_, err := pool.GetClient()
			if !errors.Is(err, clientpool.ErrExhausted) {
				t.Errorf("Expected GetClient to return exhausted error, got %v", err)
			}
		},
	)
}
