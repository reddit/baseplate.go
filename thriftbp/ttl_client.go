package thriftbp

import (
	"time"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/reddit/baseplate.go/randbp"
)

// DefaultMaxConnectionAge is the default max age for a Thrift client connection.
const DefaultMaxConnectionAge = time.Minute * 5

// DefaultMaxConnectionAgeJitter is the default jitter to MaxConnectionAge for a
// Thrift client connection.
const DefaultMaxConnectionAgeJitter = 0.1

var _ Client = (*ttlClient)(nil)

// ttlClient is a Client implementation wrapping thrift's TTransport with a TTL.
type ttlClient struct {
	thrift.TClient

	transport thrift.TTransport

	// if expiration is zero, then the client will be kept open indefinetly.
	expiration time.Time
}

// Close implements Client interface.
//
// It calls underlying TTransport's Close function.
func (c *ttlClient) Close() error {
	return c.transport.Close()
}

// IsOpen implements Client interface.
//
// If TTL has passed, it closes the underlying TTransport and returns false.
// Otherwise it just calls the underlying TTransport's IsOpen function.
func (c *ttlClient) IsOpen() bool {
	if !c.transport.IsOpen() {
		return false
	}
	if !c.expiration.IsZero() && time.Now().After(c.expiration) {
		c.transport.Close()
		return false
	}
	return true
}

// newTTLClient creates a ttlClient with a thrift TTransport and a ttl+jitter.
func newTTLClient(transport thrift.TTransport, client thrift.TClient, ttl time.Duration, jitter float64) *ttlClient {
	var expiration time.Time
	if ttl == 0 {
		ttl = DefaultMaxConnectionAge
	}
	if ttl > 0 {
		expiration = time.Now().Add(randbp.JitterDuration(ttl, jitter))
	}
	return &ttlClient{
		TClient:    client,
		transport:  transport,
		expiration: expiration,
	}
}
