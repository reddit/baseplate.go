package thriftbp

import (
	"context"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/avast/retry-go"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/reddit/baseplate.go/prometheusbp"
	"github.com/reddit/baseplate.go/randbp"
	"github.com/reddit/baseplate.go/retrybp"
)

type ttlClientGenerator func() (thrift.TClient, thrift.TTransport, error)

// DefaultMaxConnectionAge is the default max age for a Thrift client connection.
const DefaultMaxConnectionAge = time.Minute * 5

// DefaultMaxConnectionAgeJitter is the default jitter to MaxConnectionAge for a
// Thrift client connection.
const DefaultMaxConnectionAgeJitter = 0.1

// refresh related constants
const (
	ttlClientRefreshInitialDelay = 100 * time.Millisecond
	ttlClientRefreshMaxJitter    = 100 * time.Millisecond
	ttlClientRefreshMaxDelay     = 30 * time.Second

	// NOTE: This const is also used to define the buckets used by
	// ttlClientRefreshAttemptsHisto, so take care when changing it.
	ttlClientRefreshAttempts = 10
)

var _ Client = (*ttlClient)(nil)

type ttlClientState struct {
	client        thrift.TClient
	transport     thrift.TTransport
	expiration    time.Time // if expiration is zero, then the client will be kept open indefinetly.
	timer         *time.Timer
	closed        bool
	cancelRefresh context.CancelFunc // non-nil means a refresh is in progress and can be canceled by this
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
	slug      string

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
	if state.cancelRefresh != nil {
		state.cancelRefresh()
	}
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
	// set up ctx for retry cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if closed := func(cancel context.CancelFunc) bool {
		// Use a lambda to guard access to state to set cancelRefresh and return closed
		state := <-c.state
		defer func() {
			c.state <- state
		}()
		state.cancelRefresh = cancel
		return state.closed
	}(cancel); closed {
		// In a rare race condition, it's possible that the timer for refresh fired
		// at the time Close is called. In such case, return early here to avoid
		// getting into a very long retry loop later with no way to cancel.
		return
	}

	var client thrift.TClient
	var transport thrift.TTransport
	var attempts int
	defer func() {
		ttlClientRefreshAttemptsHisto.With(prometheus.Labels{
			clientNameLabel: c.slug,
		}).Observe(float64(attempts))
	}()
	if retrybp.Do(
		ctx,
		func() error {
			attempts++
			var err error
			client, transport, err = c.generator()
			if err != nil {
				ttlClientReplaceCounter.With(prometheus.Labels{
					clientNameLabel: c.slug,
					successLabel:    prometheusbp.BoolString(false),
				}).Inc()
			}
			return err
		},
		retry.Attempts(ttlClientRefreshAttempts),
		retrybp.CappedExponentialBackoff(retrybp.CappedExponentialBackoffArgs{
			InitialDelay: ttlClientRefreshInitialDelay,
			MaxJitter:    ttlClientRefreshMaxJitter,
			MaxDelay:     ttlClientRefreshMaxDelay,
		}),
		retrybp.Filters(
			retrybp.RetryableErrorFilter,
			retrybp.NetworkErrorFilter,
		),
	) != nil {
		// We cannot replace this connection in the background,
		// leave client and transport be,
		// this connection will be replaced by the pool upon next use.
		return
	}

	// replace with the refreshed connection
	state := <-c.state
	defer func() {
		c.state <- state
	}()
	state.cancelRefresh = nil
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
	ttlClientReplaceCounter.With(prometheus.Labels{
		clientNameLabel: c.slug,
		successLabel:    prometheusbp.BoolString(true),
	}).Inc()
}

// newTTLClient creates a ttlClient with a thrift TTransport and ttl+jitter.
func newTTLClient(generator ttlClientGenerator, ttl time.Duration, jitter float64, slug string) (*ttlClient, error) {
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
		slug:      slug,

		state: make(chan *ttlClientState, 1),
	}
	state := &ttlClientState{
		client:    client,
		transport: transport,
	}
	state.renew(time.Now(), c)
	c.state <- state

	// Register the error counter so it can be monitored
	ttlClientReplaceCounter.With(prometheus.Labels{
		clientNameLabel: c.slug,
		successLabel:    prometheusbp.BoolString(false),
	})

	return c, nil
}
