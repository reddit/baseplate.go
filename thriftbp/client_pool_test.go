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

func TestNewBaseplateClientPool(t *testing.T) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	if _, err = thriftbp.NewBaseplateClientPool(
		thriftbp.ClientPoolConfig{
			Addr:               ln.Addr().String(),
			ServiceSlug:        "test",
			InitialConnections: 1,
			MaxConnections:     5,
			MaxConnectionAge:   time.Minute,
			SocketTimeout:      time.Millisecond * 10,
		},
	); err != nil {
		t.Fatal(err)
	}
}

func TestCustomClientPool(t *testing.T) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	if _, err := thriftbp.NewCustomClientPool(
		thriftbp.ClientPoolConfig{
			ServiceSlug:        "test",
			InitialConnections: 1,
			MaxConnections:     5,
			SocketTimeout:      time.Millisecond * 10,
		},
		thriftbp.SingleAddressGenerator(ln.Addr().String()),
		thrift.NewTBinaryProtocolFactoryDefault(),
	); err != nil {
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

			if err := pool.Call(context.Background(), "test", nil, nil); err != nil {
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

			err := pool.Call(context.Background(), "test", nil, nil)
			if !errors.Is(err, clientpool.ErrExhausted) {
				t.Errorf("Expected returned error to wrap exhausted error, got %v", err)
			}
		},
	)
}
