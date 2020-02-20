package thriftclient_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/reddit/baseplate.go/thriftclient"
)

const (
	addr = ":0"
)

func TestSingleAddressGenerator(t *testing.T) {
	t.Parallel()

	gen := thriftclient.SingleAddressGenerator(addr)
	generated := gen()
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
			Addr:           ln.Addr().String(),
			ServiceSlug:    "test",
			MinConnections: 1,
			MaxConnections: 5,
			SocketTimeout:  time.Millisecond * 10,
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
		ServiceSlug:    "test",
		MinConnections: 1,
		MaxConnections: 5,
		SocketTimeout:  time.Millisecond * 10,
	}
	_, _, client := initClients()
	fact := func(trans thrift.TTransport, _ thrift.TProtocolFactory) thriftclient.Client {
		return thriftclient.NewTTLClient(trans, client, time.Minute)
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
