package clientpool

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/reddit/baseplate.go/log"
)

type channelPool struct {
	pool           chan Client
	opener         ClientOpener
	numActive      atomic.Int32
	initialClients int
	maxClients     int
}

// Make sure channelPool implements Pool interface.
var _ Pool = (*channelPool)(nil)

// NewChannelPool creates a new client pool implemented via channel.
//
// Note that this function could return both non-nil Pool and error,
// when we failed to create all asked bestEffortInitialClients.
// In such case the returned Pool would have at least requiredInitialClients
// clients we already established.
func NewChannelPool(ctx context.Context, requiredInitialClients, bestEffortInitialClients, maxClients int, opener ClientOpener) (Pool, error) {
	if !(requiredInitialClients <= bestEffortInitialClients && bestEffortInitialClients <= maxClients) {
		return nil, &ConfigError{
			BestEffortInitialClients: bestEffortInitialClients,
			RequiredInitialClients:   requiredInitialClients,
			MaxClients:               maxClients,
		}
	}

	var lastAttemptErr error
	pool := make(chan Client, maxClients)

	for i := 0; i < requiredInitialClients; {
		if ctxErr := ctx.Err(); ctxErr != nil {
			if lastAttemptErr == nil {
				// In case the user sets a deadline so short that we don't have
				// time to open all the client serially despite all of them working.
				// In that case lastAttempErr would be nil so we need to indicate to
				// the user that their timeout being too short is the issue.
				lastAttemptErr = ctxErr
			}
			return &channelPool{
				pool:           pool,
				opener:         opener,
				initialClients: len(pool),
				maxClients:     maxClients,
			}, lastAttemptErr
		}
		c, err := opener()
		if err == nil {
			pool <- c
			i++
		} else {
			lastAttemptErr = err
			log.Warnf("clientpool: error creating required client (will retry): %w", err)
		}
	}
	lastAttemptErr = nil

	for i := requiredInitialClients; i < bestEffortInitialClients; i++ {
		c, err := opener()
		if err != nil {
			lastAttemptErr = fmt.Errorf(
				"clientpool: error creating client #%d/%d: %w",
				i,
				bestEffortInitialClients,
				err,
			)
			break
		}
		pool <- c
	}

	return &channelPool{
		pool:           pool,
		opener:         opener,
		initialClients: bestEffortInitialClients,
		maxClients:     maxClients,
	}, lastAttemptErr
}

// Get returns a client from the pool.
func (cp *channelPool) Get() (client Client, err error) {
	defer func() {
		if err == nil {
			cp.numActive.Add(1)
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
	defer cp.numActive.Add(-1)

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
	return cp.numActive.Load()
}

// NumAllocated returns the number of allocated clients in internal pool.
func (cp *channelPool) NumAllocated() int32 {
	return int32(len(cp.pool))
}

// IsExhausted returns true when NumActiveClients >= max capacity.
func (cp *channelPool) IsExhausted() bool {
	return cp.NumActiveClients() >= int32(cp.maxClients)
}
