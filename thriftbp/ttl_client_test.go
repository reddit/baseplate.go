package thriftbp

import (
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
)

func TestTTLClient(t *testing.T) {
	transport := thrift.NewTMemoryBuffer()
	factory := thrift.NewTBinaryProtocolFactoryConf(nil)
	tc := thrift.NewTStandardClient(
		factory.GetProtocol(transport),
		factory.GetProtocol(transport),
	)
	ttl := time.Millisecond
	jitter := 0.1

	client := newTTLClient(transport, tc, ttl, jitter)
	if !client.IsOpen() {
		t.Error("Expected immediate IsOpen call to return true, got false.")
	}

	time.Sleep(ttl + time.Duration(float64(ttl)*jitter))
	if client.IsOpen() {
		t.Error("Expected IsOpen call after sleep to return false, got true.")
	}

	client = newTTLClient(transport, tc, ttl, -jitter)
	if !client.IsOpen() {
		t.Error("Expected immediate IsOpen call to return true, got false.")
	}

	time.Sleep(ttl)
	if client.IsOpen() {
		t.Error("Expected IsOpen call after sleep to return false, got true.")
	}
}

func TestTTLClientNegativeTTL(t *testing.T) {
	transport := thrift.NewTMemoryBuffer()
	factory := thrift.NewTBinaryProtocolFactoryConf(nil)
	tc := thrift.NewTStandardClient(
		factory.GetProtocol(transport),
		factory.GetProtocol(transport),
	)
	ttl := time.Millisecond

	client := newTTLClient(transport, tc, -ttl, 0.1)
	if !client.IsOpen() {
		t.Error("Expected immediate IsOpen call to return true, got false.")
	}

	time.Sleep(ttl)
	if !client.IsOpen() {
		t.Error("Expected IsOpen call after sleep to return true, got false.")
	}
}
