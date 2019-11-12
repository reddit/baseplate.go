package thriftpool

import (
	"sync/atomic"
)

type channelPool struct {
	pool           chan Client
	opener         ClientOpener
	numActive      int32
	minConnections int
	maxConnections int
}

// Make sure channelPool implements Pool interface.
var _ Pool = (*channelPool)(nil)

// NewChannelPool creates a new thrift client pool implemented via channel.
func NewChannelPool(minConnections, maxConnections int, opener ClientOpener) (Pool, error) {
	if minConnections > maxConnections {
		return nil, &ConfigError{
			MinConnections: minConnections,
			MaxConnections: maxConnections,
		}
	}

	pool := make(chan Client, maxConnections)
	for i := 0; i < minConnections; i++ {
		c, err := opener()
		if err != nil {
			return nil, err
		}
		pool <- c
	}

	return &channelPool{
		pool:           pool,
		opener:         opener,
		minConnections: minConnections,
		maxConnections: maxConnections,
	}, nil
}

// Get returns a thrift client from the pool.
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

	if !c.IsOpen() {
		newC, err := cp.opener()
		if err != nil {
			return err
		}
		c = newC
	}

	select {
	case cp.pool <- c:
		atomic.AddInt32(&cp.numActive, -1)
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
	return cp.NumActiveClients() >= int32(cp.maxConnections)
}
