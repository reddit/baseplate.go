package thriftbp_test

import (
	"net"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
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
			ConnectTimeout:     time.Millisecond * 5,
			SocketTimeout:      time.Millisecond * 15,
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
			ConnectTimeout:     time.Millisecond * 5,
			SocketTimeout:      time.Millisecond * 15,
		},
		thriftbp.SingleAddressGenerator(ln.Addr().String()),
		thrift.NewTBinaryProtocolFactoryDefault(),
	); err != nil {
		t.Fatal(err)
	}
}
