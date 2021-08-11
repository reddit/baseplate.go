package redisx

import (
	"context"

	"github.com/joomcode/redispipe/redis"
)

// Sync is the interface used Syncx.
//
// redispipe's SyncCtx almost implements this interface, the difference is its
// Scanner function returns a distinct type rather than a ScanIterator.
// BaseSync has been provided as an adapter around redis.SyncCtx to make it
// conform to the Sync interface defined here.
type Sync interface {
	// Do executes a single redis command with the given args.
	Do(ctx context.Context, cmd string, args ...interface{}) interface{}

	// Send executes the given redis.Request.
	Send(ctx context.Context, req redis.Request) interface{}

	// SendMany executes all of the given requests in a single, non-transactional
	// pipeline.
	SendMany(ctx context.Context, reqs []redis.Request) []interface{}

	// SendTransaction executes al of the given requests in a single, transactional
	// pipeline.
	SendTransaction(ctx context.Context, reqs []redis.Request) ([]interface{}, error)

	// Scanner returns an iterator over the results from running a SCAN with the
	// given redis.ScanOpts.
	Scanner(ctx context.Context, opts redis.ScanOpts) ScanIterator
}

// ScanIterator iterates over the results of a SCAN call.
type ScanIterator interface {
	Next() ([]string, error)
}

var (
	_ ScanIterator = redis.SyncCtxIterator{}
)

// BaseSync wraps SyncCtx from redispipe, replacing it's "Scanner" method with
// one that implements the definition in the SyncCtx interface defined in
// "redispipebp".
type BaseSync struct {
	redis.SyncCtx
}

// Scanner returns s.SyncCtx.Scanner as a ScanIterator rather than redis.SyncCtxIterator. This
// allows it to implement the Sync interface.
func (s BaseSync) Scanner(ctx context.Context, opts redis.ScanOpts) ScanIterator {
	return s.SyncCtx.Scanner(ctx, opts)
}

var (
	_ Sync = BaseSync{}
)
