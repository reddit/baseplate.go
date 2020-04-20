package thriftbp_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/reddit/baseplate.go/clientpool"
	"github.com/reddit/baseplate.go/thriftbp"
)

const (
	addr = ":0"
)

func TestSingleAddressGenerator(t *testing.T) {
	t.Parallel()

	gen := thriftbp.SingleAddressGenerator(addr)
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

	fact := thriftbp.NewTTLClientFactory(time.Millisecond * 3)
	trans := thrift.NewTMemoryBuffer()
	protoFactory := thrift.NewTBinaryProtocolFactoryDefault()
	tClientFactory := thriftbp.StandardTClientFactory
	client := fact(tClientFactory, trans, protoFactory)
	if _, ok := client.(*thriftbp.TTLClient); !ok {
		t.Fatal("wrong type for client")
	}
}

func TestStandardTClientFactory(t *testing.T) {
	t.Parallel()

	trans := thrift.NewTMemoryBuffer()
	protoFact := thrift.NewTBinaryProtocolFactoryDefault()
	c := thriftbp.StandardTClientFactory(trans, protoFact)
	if _, ok := c.(*thrift.TStandardClient); !ok {
		t.Fatal("wrong type for client")
	}
}

func testClientMiddleware(c *counter) thriftbp.ClientMiddleware {
	return func(next thrift.TClient) thrift.TClient {
		return thriftbp.WrappedTClient{
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
	tClientFactory := thriftbp.NewWrappedTClientFactory(
		thriftbp.NewMockTClientFactory(thriftbp.MockClient{}),
		testClientMiddleware(count),
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

	pool, err := thriftbp.NewBaseplateClientPool(
		thriftbp.ClientPoolConfig{
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

	pool, err := thriftbp.NewCustomClientPool(
		thriftbp.ClientPoolConfig{
			ServiceSlug:        "test",
			InitialConnections: 1,
			MaxConnections:     5,
			SocketTimeout:      time.Millisecond * 10,
		},
		thriftbp.SingleAddressGenerator(ln.Addr().String()),
		func(thriftbp.TClientFactory, thrift.TTransport, thrift.TProtocolFactory) thriftbp.Client {
			return &thriftbp.MockClient{}
		},
		thriftbp.StandardTClientFactory,
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
	pool := thriftbp.MockClientPool{}

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
