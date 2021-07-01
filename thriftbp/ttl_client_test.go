package thriftbp

import (
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
)

func TestTTLClient(t *testing.T) {
	trans := thrift.NewTMemoryBuffer()
	factory := thrift.NewTBinaryProtocolFactoryConf(nil)
	tc := thrift.NewTStandardClient(
		factory.GetProtocol(trans),
		factory.GetProtocol(trans),
	)
	ttl := time.Millisecond

	client := newTTLClient(trans, tc, ttl)
	if !client.IsOpen() {
		t.Error("Expected immediate IsOpen call to return true, got false.")
	}

	time.Sleep(ttl)
	if client.IsOpen() {
		t.Error("Expected IsOpen call after sleep to return false, got true.")
	}
}
