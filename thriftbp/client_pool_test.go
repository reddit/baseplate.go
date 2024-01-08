package thriftbp_test

import (
	"context"
	"errors"
	"fmt"
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

func TestBehaviorWithNetworkIssues(t *testing.T) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	cfg := thriftbp.ClientPoolConfig{
		ServiceSlug:     "test",
		EdgeContextImpl: ecinterface.Mock(),
		ConnectTimeout:  time.Millisecond * 5,
		SocketTimeout:   time.Millisecond * 15,

		RequiredInitialConnections: 2,
		InitialConnections:         5,
		MaxConnections:             10,
	}

	for _, c := range []struct {
		label    string
		addrGen  thriftbp.AddressGenerator
		validate func(thriftbp.ClientPool, error)
	}{
		{
			label: "network-fails-once-before-required",
			addrGen: func() thriftbp.AddressGenerator {
				i := 0
				return func() (string, error) {
					i += 1
					var err error
					if i == 1 {
						err = fmt.Errorf("something broke")
					}
					return ln.Addr().String(), err
				}
			}(),
			validate: func(p thriftbp.ClientPool, err error) {
				if err != nil {
					t.Errorf("Didn't expect an error but got %v", err)
				}
			},
		},
		{
			label: "network-fails-once-after-required",
			addrGen: func() thriftbp.AddressGenerator {
				i := 0
				return func() (string, error) {
					i += 1
					var err error
					if i == 4 {
						err = fmt.Errorf("something broke")
					}
					return ln.Addr().String(), err
				}
			}(),
			validate: func(p thriftbp.ClientPool, err error) {
				if err != nil {
					t.Errorf("Didn't expect an error but got %v", err)
				}
			},
		},
		{
			label: "network-fails-consistently-before-required",
			addrGen: func() thriftbp.AddressGenerator {
				i := 0
				return func() (string, error) {
					i += 1
					var err error
					if i >= 2 {
						err = fmt.Errorf("something broke")
					}
					return ln.Addr().String(), err
				}
			}(),
			validate: func(p thriftbp.ClientPool, err error) {
				if err == nil {
					t.Errorf("Expected an error.")
				}
			},
		},
		{
			label: "network-fails-consistently-after-required",
			addrGen: func() thriftbp.AddressGenerator {
				i := 0
				return func() (string, error) {
					i += 1
					var err error
					if i >= 4 {
						err = fmt.Errorf("something broke")
					}
					return ln.Addr().String(), err
				}
			}(),
			validate: func(p thriftbp.ClientPool, err error) {
				if err != nil {
					t.Errorf("Didn't expect an error but got %v", err)
				}
			},
		},
	} {
		t.Run(c.label, func(t *testing.T) {
			factory := thrift.NewTBinaryProtocolFactoryConf(cfg.ToTConfiguration())
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
			defer cancel()
			pool, err := thriftbp.NewCustomClientPoolWithContext(ctx, cfg, c.addrGen, factory)
			c.validate(pool, err)
		})
	}
}

func TestThriftHostnameHeader(t *testing.T) {
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
	pool, err := thriftbp.NewCustomClientPool(
		cfg,
		thriftbp.SingleAddressGenerator(ln.Addr().String()),
		thrift.NewTBinaryProtocolFactoryConf(cfg.ToTConfiguration()),
	)
	if err != nil {
		t.Fatal(err)
	}

	go func() { ln.Accept() }()
	pool.TClient().Call(context.Background(), "test", nil, nil)
	// conn, _ := ln.Accept()
	// buff := make([]byte, 1024)
	// conn.Read(buff)
	fmt.Println("string(buff)")
}

func TestInitialConnectionsFallback(t *testing.T) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	var counter atomic.Uint64
	addrGen := func() (string, error) {
		if counter.Add(1)%2 == 0 {
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
			}
		})
	}
}
