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

func TestTTLClientFactory(t *testing.T) {
	t.Parallel()

	fact := thriftclient.NewTTLClientFactory(time.Millisecond * 3)
	trans := thrift.NewTMemoryBuffer()
	protoFactory := thrift.NewTBinaryProtocolFactoryDefault()
	tClientFactory := thriftclient.StandardTClientFactory
	client := fact(tClientFactory, trans, protoFactory)
	if _, ok := client.(*thriftclient.TTLClient); !ok {
		t.Fatal("wrong type for client")
	}
}

func TestStandardTClientFactory(t *testing.T) {
	t.Parallel()

	trans := thrift.NewTMemoryBuffer()
	protoFact := thrift.NewTBinaryProtocolFactoryDefault()
	c := thriftclient.StandardTClientFactory(trans, protoFact)
	if _, ok := c.(*thrift.TStandardClient); !ok {
		t.Fatal("wrong type for client")
	}
}

type counter struct {
	count int
}

func (c *counter) incr() {
	c.count++
}

func testMiddleware(c *counter) thriftclient.Middleware {
	return func(next thrift.TClient) thrift.TClient {
		return thriftclient.WrappedTClient{
			Wrapped: func(ctx context.Context, method string, args, result thrift.TStruct) error {
				c.incr()
				return next.Call(ctx, method, args, result)
			},
		}
	}
}

func TestNewWrappedTClientFactory(t *testing.T) {
	t.Parallel()

	count := &counter{}
	tClientFactory := thriftclient.NewWrappedTClientFactory(
		thriftclient.NewMockTClientFactory(thriftclient.MockClient{}),
		testMiddleware(count),
	)
	trans := thrift.NewTMemoryBuffer()
	protoFactory := thrift.NewTBinaryProtocolFactoryDefault()
	c := tClientFactory(trans, protoFactory)
	if count.count != 0 {
		t.Errorf("Wrong count value: %d, expected 0", count.count)
	}
	_ = c.Call(context.Background(), "test", nil, nil)
	if count.count != 1 {
		t.Errorf("Wrong count value: %d, expected 1", count.count)
	}
}

func TestNewBaseplateClientPool(t *testing.T) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	pool, err := thriftclient.NewBaseplateClientPool(
		thriftclient.ClientPoolConfig{
			Addr:               ln.Addr().String(),
			ServiceSlug:        "test",
			InitialConnections: 1,
			MaxConnections:     5,
			SocketTimeout:      time.Millisecond * 10,
		}, time.Minute,
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

	pool, err := thriftclient.NewCustomClientPool(
		thriftclient.ClientPoolConfig{
			ServiceSlug:        "test",
			InitialConnections: 1,
			MaxConnections:     5,
			SocketTimeout:      time.Millisecond * 10,
		},
		thriftclient.SingleAddressGenerator(ln.Addr().String()),
		func(thriftclient.TClientFactory, thrift.TTransport, thrift.TProtocolFactory) thriftclient.Client {
			return &thriftclient.MockClient{}
		},
		thriftclient.StandardTClientFactory,
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
