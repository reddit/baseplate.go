package thriftpool_test

import (
	"testing"
	"time"

	"github.com/reddit/baseplate.go/thriftpool"

	"github.com/apache/thrift/lib/go/thrift"
)

func TestTTLClient(t *testing.T) {
	trans := thrift.NewTMemoryBuffer()
	ttl := time.Millisecond

	client := thriftpool.NewTTLClient(trans, ttl)
	if !client.IsOpen() {
		t.Error("Expected immediate IsOpen call to return true, got false.")
	}

	time.Sleep(ttl)
	if client.IsOpen() {
		t.Error("Expected IsOpen call after sleep to return false, got true.")
	}
}
