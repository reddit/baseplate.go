package detach

import (
	"context"
	"sync"
	"time"
)

// Hooks are functions that are called with specific detach helpers to attach
// values from the parent context to the new context. In order to be registered,
//at least one function in Hooks must be non-nil. Nil functions will be skipped.
type Hooks struct {
	// Inline functions are used to attach values from the src context to the dst
	// context in calls to Inline.
	//
	// Inline functions should assume that the calling function will wait for the
	// any calls after Inline to return before it returns, so they may just
	// copy values from src to dst.
	Inline func(dst, src context.Context) context.Context

	// Async functions wrap the run methods passed to Async. They should derive
	// values from the src context to add to the dst context and do any
	// setup/cleanup around those values.
	//
	// Async functions should assume that the calling function could return before
	// they finish, so if context values on src could be closed or otherwise
	// cleaned up, the wrapper should likely create new values and clean those up
	// rather than just copying the values from src to dst.
	Async func(dst, src context.Context, next func(ctx context.Context))
}

type hooksRegistry struct {
	mu    sync.RWMutex
	hooks []Hooks
}

func (r *hooksRegistry) add(hooks Hooks) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.hooks = append(r.hooks, hooks)
}

func (r *hooksRegistry) allHooks() []Hooks {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.hooks
}

var registry = new(hooksRegistry)

// Register adds a new Hooks to the library's registry. If all functions in the
// Hooks are nil, Register will panic.
func Register(hooks Hooks) {
	if hooks.Inline == nil && hooks.Async == nil {
		panic("cannot register Hooks with all nil functions")
	}
	registry.add(hooks)
}

// Inline returns a new context who inherits the values attached using the
// Inline Hooks registered with the library from the parent but replaces the
// parent's timeout/cancel with the timeout given and the returned cancel func.
//
// Inline should be used when you need to run something within the current call,
// rather than in a new goroutine, but need to ignore any timeouts/cancellations
// from the parent context, such as cleaning up after a call that might have
// been timed out or canceled.
func Inline(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	for _, hooks := range registry.allHooks() {
		if hooks.Inline != nil {
			ctx = hooks.Inline(ctx, parent)
		}
	}
	return ctx, cancel
}

// Async wraps run in the Async Hooks registered with the library, creates a new
// context, and calls the wrapped run function with that context. The wrappers
// may attach values from the parent context to the new context and perform any
//cleanup tasks after run completes.
//
// Async should be used when you want to run an async task as a part of another
// all, but need to keep some values from the parent context.
func Async(parent context.Context, run func(ctx context.Context)) {
	ctx := context.Background()

	allHooks := registry.allHooks()
	for i := len(allHooks) - 1; i >= 0; i-- {
		hooks := allHooks[i]
		if hooks.Async != nil {
			// assign 'next' to an inner variable rather than using 'run! directly in
			// the wrapped call to avoid an infinite loop.
			next := run
			run = func(ctx context.Context) {
				// if you just used 'run' in here, it would always use the last value of
				// "run", so it would call itself over and over again, creating an
				// infinite loop.
				hooks.Async(ctx, parent, next)
			}
		}
	}
	run(ctx)
}
