package thriftbp

import (
	"context"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/go-kit/kit/metrics"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/reddit/baseplate.go/metricsbp"
	"github.com/reddit/baseplate.go/randbp"
)

type ttlClientGenerator func() (thrift.TClient, thrift.TTransport, error)

// DefaultMaxConnectionAge is the default max age for a Thrift client connection.
const DefaultMaxConnectionAge = time.Minute * 5

// DefaultMaxConnectionAgeJitter is the default jitter to MaxConnectionAge for a
// Thrift client connection.
const DefaultMaxConnectionAgeJitter = 0.1

var _ Client = (*ttlClient)(nil)

type ttlClientState struct {
	client     thrift.TClient
	transport  thrift.TTransport
	expiration time.Time // if expiration is zero, then the client will be kept open indefinetly.
	timer      *time.Timer
	closed     bool
}

// renew updates expiration and timer in s base on the given timestamp and
// client.
func (s *ttlClientState) renew(now time.Time, client *ttlClient) {
	if client.ttl < 0 {
		return
	}
	s.expiration = now.Add(client.ttl)
	s.timer = time.AfterFunc(client.ttl, client.refresh)
}

// ttlClient is a Client implementation wrapping thrift's TTransport with a TTL.
type ttlClient struct {
	// configs needed for refresh to work
	generator ttlClientGenerator
	ttl       time.Duration

	replaceCounter metrics.Counter
	slug           string

	// state guarded by lock (buffer-1 channel)
	state chan *ttlClientState
}

// Close implements Client interface.
//
// It calls underlying TTransport's Close function.
func (c *ttlClient) Close() error {
	state := <-c.state
	defer func() {
		c.state <- state
	}()
	state.closed = true
	if state.timer != nil {
		state.timer.Stop()
	}
	return state.transport.Close()
}

func (c *ttlClient) Call(ctx context.Context, method string, args, result thrift.TStruct) (thrift.ResponseMeta, error) {
	state := <-c.state
	defer func() {
		c.state <- state
	}()
	return state.client.Call(ctx, method, args, result)
}

// IsOpen implements Client interface.
//
// It checks underlying TTransport's IsOpen first,
// if that returns false, it returns false.
// Otherwise it checks TTL,
// returns false if TTL has passed and also close the underlying TTransport.
func (c *ttlClient) IsOpen() bool {
	state := <-c.state
	defer func() {
		c.state <- state
	}()
	if !state.transport.IsOpen() {
		return false
	}
	if !state.expiration.IsZero() && time.Now().After(state.expiration) {
		state.transport.Close()
		return false
	}
	return true
}

// refresh is called when the ttl hits to try to refresh the connection.
func (c *ttlClient) refresh() {
	client, transport, err := c.generator()
	if err != nil {
		// We cannot replace this connection in the background,
		// leave client and transport be,
		// this connection will be replaced by the pool upon next use.
		c.replaceCounter.With("success", "False").Add(1)
		ttlClientReplaceCounter.With(prometheus.Labels{
			serverSlugLabel: c.slug,
			successLabel:    "false",
		}).Inc()
		return
	}

	// replace with the refreshed connection
	state := <-c.state
	defer func() {
		c.state <- state
	}()
	if state.closed {
		// If Close was called after we entered this function,
		// close the newly created connection and return early.
		transport.Close()
		return
	}
	state.renew(time.Now(), c)
	state.client = client
	if state.transport != nil {
		// close the old transport before replacing it, to avoid connection leaks.
		state.transport.Close()
	}
	state.transport = transport
	c.replaceCounter.With("success", "True").Add(1)
	ttlClientReplaceCounter.With(prometheus.Labels{
		serverSlugLabel: c.slug,
		successLabel:    "true",
	}).Inc()
}

// newTTLClient creates a ttlClient with a thrift TTransport and ttl+jitter.
func newTTLClient(generator ttlClientGenerator, ttl time.Duration, jitter float64, slug string, tags metricsbp.Tags) (*ttlClient, error) {
	client, transport, err := generator()
	if err != nil {
		return nil, err
	}

	if ttl == 0 {
		ttl = DefaultMaxConnectionAge
	}
	duration := randbp.JitterDuration(ttl, jitter)
	c := &ttlClient{
		generator: generator,
		ttl:       duration,

		replaceCounter: metricsbp.M.Counter(slug + ".connection-housekeeping").With(tags.AsStatsdTags()...),
		slug:           slug,

		state: make(chan *ttlClientState, 1),
	}
	state := &ttlClientState{
		client:    client,
		transport: transport,
	}
	state.renew(time.Now(), c)
	c.state <- state
	return c, nil
}
