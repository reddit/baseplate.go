package thriftbp

import (
	"time"

	"github.com/apache/thrift/lib/go/thrift"
)

var _ Client = (*TTLClient)(nil)

// TTLClient is a Client implementation wrapping thrift's TTransport with a TTL.
//
// It's intended to be managed by a ClientPool rather than created directly.
type TTLClient struct {
	thrift.TClient

	trans      thrift.TTransport
	expiration time.Time
}

// Close implements Client interface.
//
// It calls underlying TTransport's Close function.
func (c *TTLClient) Close() error {
	return c.trans.Close()
}

// IsOpen implements Client interface.
//
// If TTL has passed, it closes the underlying TTransport and returns false.
// Otherwise it just calls the underlying TTransport's IsOpen function.
func (c *TTLClient) IsOpen() bool {
	if !c.trans.IsOpen() {
		return false
	}
	if time.Now().After(c.expiration) {
		c.trans.Close()
		return false
	}
	return true
}

// NewTTLClient creates a TTLClient with a thrift TTransport and a ttl.
func NewTTLClient(trans thrift.TTransport, client thrift.TClient, ttl time.Duration) *TTLClient {
	return &TTLClient{
		TClient:    client,
		trans:      trans,
		expiration: time.Now().Add(ttl),
	}
}
