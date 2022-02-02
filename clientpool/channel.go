package clientpool

import (
	"fmt"
	"sync/atomic"
)

type channelPool struct {
	pool           chan Client
	opener         ClientOpener
	numActive      int32
	initialClients int
	maxClients     int
}

// Make sure channelPool implements Pool interface.
var _ Pool = (*channelPool)(nil)

// NewChannelPool creates a new client pool implemented via channel.
func NewChannelPool(initialClients, maxClients int, opener ClientOpener) (Pool, error) {
	if initialClients > maxClients {
		return nil, &ConfigError{
			InitialClients: initialClients,
			MaxClients:     maxClients,
		}
	}

	pool := make(chan Client, maxClients)
	for i := 0; i < initialClients; i++ {
		c, err := opener()
		if err != nil {
			return nil, fmt.Errorf(
				"error creating client #%d/%d: %w",
				i,
				initialClients,
				err,
			)
		}
		pool <- c
	}

	return &channelPool{
		pool:           pool,
		opener:         opener,
		initialClients: initialClients,
		maxClients:     maxClients,
	}, nil
}

// Get returns a client from the pool.
func (cp *channelPool) Get() (client Client, err error) {
	defer func() {
		if err == nil {
			atomic.AddInt32(&cp.numActive, 1)
		}
	}()

	select {
	case c := <-cp.pool:
		if c.IsOpen() {
			return c, nil
		}
		// For thrift connections, IsOpen could return false in both explicit and
		// implicit closed situations.
		// In implicit closed situation, IsOpen does a connectivity check and
		// returns false if that check fails. In such case we should still close the
		// connection explicitly to avoid resource leak.
		// In explicit situation, calling Close again will just return an already
		// closed error, which is harmless here.
		c.Close()
	default:
	}

	if cp.IsExhausted() {
		err = ErrExhausted
		return
	}
	return cp.opener()
}

// Release releases a client back to the pool.
//
// If the pool is full, the client will be closed instead.
//
// Calling Release after Close will cause panic.
func (cp *channelPool) Release(c Client) error {
	if c == nil {
		return nil
	}

	// As long as c is not nil, we always need to decrease numActive by 1,
	// even if we encounter errors here, either due to close or opener.
	defer atomic.AddInt32(&cp.numActive, -1)

	if !c.IsOpen() {
		// Even when c.IsOpen reported false, still call Close explicitly to avoid
		// connection leaks. At worst case scenario it just returns an already
		// closed error, which is still harmless.
		c.Close()

		newC, err := cp.opener()
		if err != nil {
			return err
		}
		c = newC
	}

	select {
	case cp.pool <- c:
		return nil
	default:
		// Pool is full, just close it instead.
		return c.Close()
	}
}

// Close closes the pool, and all allocated clients.
func (cp *channelPool) Close() error {
	var lastErr error
	close(cp.pool)
	for c := range cp.pool {
		if err := c.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// NumActiveClients returns the number of clients curently given out for use.
func (cp *channelPool) NumActiveClients() int32 {
	return atomic.LoadInt32(&cp.numActive)
}

// NumAllocated returns the number of allocated clients in internal pool.
func (cp *channelPool) NumAllocated() int32 {
	return int32(len(cp.pool))
}

// IsExhausted returns true when NumActiveClients >= max capacity.
func (cp *channelPool) IsExhausted() bool {
	return cp.NumActiveClients() >= int32(cp.maxClients)
}
