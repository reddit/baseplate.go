package thriftbp_test

import (
	"testing"
	"time"

	"github.com/reddit/baseplate.go/thriftbp"

	"github.com/apache/thrift/lib/go/thrift"
)

func TestTTLClient(t *testing.T) {
	trans := thrift.NewTMemoryBuffer()
	factory := thrift.NewTBinaryProtocolFactoryDefault()
	tc := thrift.NewTStandardClient(
		factory.GetProtocol(trans),
		factory.GetProtocol(trans),
	)
	ttl := time.Millisecond

	client := thriftbp.NewTTLClient(trans, tc, ttl)
	if !client.IsOpen() {
		t.Error("Expected immediate IsOpen call to return true, got false.")
	}

	time.Sleep(ttl)
	if client.IsOpen() {
		t.Error("Expected IsOpen call after sleep to return false, got true.")
	}
}
