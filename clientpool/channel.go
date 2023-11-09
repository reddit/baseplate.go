package clientpool

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/reddit/baseplate.go/log"
	"golang.org/x/time/rate"
)

type channelPool struct {
	pool                   chan Client
	opener                 ClientOpener
	numActive              atomic.Int32
	numTotal               atomic.Int32
	backgroundTaskInterval time.Duration
	minClients             int
	maxClients             int
	isClosed               atomic.Bool
}

const DefaultBackgroundTaskInterval = 5 * time.Second

// Make sure channelPool implements Pool interface.
var _ Pool = (*channelPool)(nil)

// NewChannelPool creates a new client pool implemented via channel.
func NewChannelPool(ctx context.Context, requiredInitialClients, bestEffortInitialClients, maxClients int, opener ClientOpener) (_ Pool, err error) {
	return NewChannelPoolWithMinClients(ctx, requiredInitialClients, bestEffortInitialClients, -1, maxClients, opener, DefaultBackgroundTaskInterval)
}

// NewChannelPoolWithMinClients creates a new client pool implemented via channel.
func NewChannelPoolWithMinClients(ctx context.Context, requiredInitialClients, bestEffortInitialClients, minClients, maxClients int, opener ClientOpener, backgroundTaskInterval time.Duration) (_ Pool, err error) {
	if !(requiredInitialClients <= bestEffortInitialClients && bestEffortInitialClients <= maxClients && minClients <= maxClients) {
		return nil, &ConfigError{
			BestEffortInitialClients: bestEffortInitialClients,
			RequiredInitialClients:   requiredInitialClients,
			MinClients:               minClients,
			MaxClients:               maxClients,
		}
	}

	var lastAttemptErr error
	pool := make(chan Client, maxClients)
	chatty := rate.NewLimiter(rate.Every(2*time.Second), 1)

	defer func() {
		if err != nil {
			// If we are returning an error, we need to make sure that we close all
			// already created connections
			close(pool)
			for c := range pool {
				c.Close()
			}
		}
	}()

	var numTotal int32

	for i := 0; i < requiredInitialClients; {
		if ctxErr := ctx.Err(); ctxErr != nil {
			if lastAttemptErr == nil {
				// In case the user sets a deadline so short that we don't have
				// time to open all the client serially despite all of them working.
				// In that case lastAttempErr would be nil so we need to indicate to
				// the user that their timeout being too short is the issue.
				lastAttemptErr = ctxErr
			}
			return nil, lastAttemptErr
		}
		c, err := opener()
		if err == nil {
			pool <- c
			i++
			numTotal++
		} else {
			lastAttemptErr = err
			if chatty.Allow() {
				log.Warnf("clientpool: error creating required client (will retry): %v", err)
			}
		}
	}

	for i := requiredInitialClients; i < bestEffortInitialClients; i++ {
		c, err := opener()
		if err != nil {
			log.Warnf(
				"clientpool: error creating best-effort client #%d/%d: %v",
				i,
				bestEffortInitialClients,
				err,
			)
			break
		}
		pool <- c
		numTotal++
	}

	if backgroundTaskInterval == 0 {
		backgroundTaskInterval = DefaultBackgroundTaskInterval
	}
	cp := &channelPool{
		pool:                   pool,
		opener:                 opener,
		maxClients:             maxClients,
		minClients:             minClients,
		backgroundTaskInterval: backgroundTaskInterval,
	}
	cp.numTotal.Store(numTotal)

	if cp.minClients > 0 {
		go cp.ensureMinClients()
	}
	return cp, nil
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

	// Instead of decrementing and re-incrementing numTotal, just decrement if we
	// failed to open a new connection to replace the closed one.
	defer func() {
		if err != nil {
			cp.numTotal.Add(-1)
		}
	}()

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
			cp.numTotal.Add(-1)
			return err
		}
		c = newC
	}

	select {
	case cp.pool <- c:
		return nil
	default:
		// Pool is full, just close it instead.
		cp.numTotal.Add(-1)
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
		cp.numTotal.Add(-1)
	}
	cp.isClosed.Store(true)
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

func (cp *channelPool) ensureMinClients() {
	for !cp.isClosed.Load() {
		time.Sleep(cp.backgroundTaskInterval)

		for cp.numTotal.Load() < int32(cp.minClients) && !cp.isClosed.Load() {
			c, err := cp.opener()
			if err != nil {
				log.Warnf("clientpool: error creating background client (will retry): %v", err)
				break
			}
			select {
			case cp.pool <- c:
				cp.numTotal.Add(1)
			default:
				// Pool is full
				c.Close()
			}
		}
	}
}
