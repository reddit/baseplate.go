package thriftbp

import (
	"time"

	"github.com/apache/thrift/lib/go/thrift"
)

// DefaultMaxConnectionAge is the default max age for a Thrift client connection.
const DefaultMaxConnectionAge = time.Minute * 5

var _ Client = (*ttlClient)(nil)

// ttlClient is a Client implementation wrapping thrift's TTransport with a TTL.
type ttlClient struct {
	thrift.TClient

	trans thrift.TTransport

	// if expiration is nil, then the client will be kept open indefinetly.
	expiration *time.Time
}

// Close implements Client interface.
//
// It calls underlying TTransport's Close function.
func (c *ttlClient) Close() error {
	return c.trans.Close()
}

// IsOpen implements Client interface.
//
// If TTL has passed, it closes the underlying TTransport and returns false.
// Otherwise it just calls the underlying TTransport's IsOpen function.
func (c *ttlClient) IsOpen() bool {
	if !c.trans.IsOpen() {
		return false
	}
	if c.expiration != nil && time.Now().After(*c.expiration) {
		c.trans.Close()
		return false
	}
	return true
}

// newTTLClient creates a ttlClient with a thrift TTransport and a ttl.
func newTTLClient(trans thrift.TTransport, client thrift.TClient, ttl time.Duration) *ttlClient {
	var expiration time.Time
	if ttl == 0 {
		ttl = DefaultMaxConnectionAge
	}
	if ttl > 0 {
		expiration = time.Now().Add(ttl)
	}
	return &ttlClient{
		TClient:    client,
		trans:      trans,
		expiration: &expiration,
	}
}
