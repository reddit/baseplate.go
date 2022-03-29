package thriftbp_test

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/reddit/baseplate.go/ecinterface"
	"github.com/reddit/baseplate.go/thriftbp"
)

const (
	addr = "localhost:0"
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
			EdgeContextImpl:    ecinterface.Mock(),
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

	cfg := thriftbp.ClientPoolConfig{
		Addr:               ":9090",
		EdgeContextImpl:    ecinterface.Mock(),
		ServiceSlug:        "test",
		InitialConnections: 1,
		MaxConnections:     5,
		ConnectTimeout:     time.Millisecond * 5,
		SocketTimeout:      time.Millisecond * 15,
	}
	if _, err := thriftbp.NewCustomClientPool(
		cfg,
		thriftbp.SingleAddressGenerator(ln.Addr().String()),
		thrift.NewTBinaryProtocolFactoryConf(cfg.ToTConfiguration()),
	); err != nil {
		t.Fatal(err)
	}
}

func TestInitialConnectionsFallback(t *testing.T) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	var counter uint64
	addrGen := func() (string, error) {
		if atomic.AddUint64(&counter, 1)%2 == 0 {
			// on even attempts, return the valid address
			return ln.Addr().String(), nil
		}
		// on odd attempts, return an error
		return "", errors.New("error")
	}

	for _, c := range []struct {
		label       string
		expectError bool
		cfg         thriftbp.ClientPoolConfig
	}{
		{
			label:       "invalid-config",
			expectError: true,
			cfg: thriftbp.ClientPoolConfig{
				ServiceSlug:     "test",
				EdgeContextImpl: ecinterface.Mock(),
				ConnectTimeout:  time.Millisecond * 5,
				SocketTimeout:   time.Millisecond * 15,

				InitialConnections: 5,
				MaxConnections:     2,
			},
		},
		{
			label:       "required-1",
			expectError: true,
			cfg: thriftbp.ClientPoolConfig{
				ServiceSlug:     "test",
				EdgeContextImpl: ecinterface.Mock(),
				ConnectTimeout:  time.Millisecond * 5,
				SocketTimeout:   time.Millisecond * 15,

				InitialConnections:         2,
				MaxConnections:             5,
				RequiredInitialConnections: 1,
			},
		},
		{
			label:       "required-0",
			expectError: false,
			cfg: thriftbp.ClientPoolConfig{
				ServiceSlug:     "test",
				EdgeContextImpl: ecinterface.Mock(),
				ConnectTimeout:  time.Millisecond * 5,
				SocketTimeout:   time.Millisecond * 15,

				InitialConnections:         2,
				MaxConnections:             5,
				RequiredInitialConnections: 0,
			},
		},
	} {
		t.Run(c.label, func(t *testing.T) {
			var loggerCalled int64
			logger := func(_ context.Context, msg string) {
				t.Logf("InitialConnectionsFallbackLogger called with %q", msg)
				atomic.StoreInt64(&loggerCalled, 1)
			}

			c.cfg.InitialConnectionsFallbackLogger = logger
			factory := thrift.NewTBinaryProtocolFactoryConf(c.cfg.ToTConfiguration())

			_, err := thriftbp.NewCustomClientPool(c.cfg, addrGen, factory)
			if c.expectError {
				t.Logf("err: %v", err)
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
				if atomic.LoadInt64(&loggerCalled) != 1 {
					t.Error("InitialConnectionsFallbackLogger not called")
				}
			}
		})
	}
}
