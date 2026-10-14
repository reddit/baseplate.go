package internalv2compat

import (
	"context"
	"sync"
	"time"

	"github.com/opentracing/opentracing-go"
)

// ServerSpan carries the real v2 span and the metadata needed by v0 span hooks.
// The returned context must contain the same span for both tracing APIs.
type ServerSpan struct {
	Span                      opentracing.Span
	TraceID, SpanID, ParentID string
	Sampled                   bool
	StartTime                 time.Time
}

// ServerSpanProvider starts a v2 server span on behalf of a legacy helper.
type ServerSpanProvider func(context.Context, string) (context.Context, ServerSpan)

var serverSpanProvider struct {
	sync.RWMutex
	provider ServerSpanProvider
}

// SetV2TracingServerSpanProvider installs the v2 implementation of legacy server
// spans. Passing nil restores legacy behavior. The previous provider is returned
// so temporary tracing providers can restore it during cleanup.
func SetV2TracingServerSpanProvider(provider ServerSpanProvider) ServerSpanProvider {
	serverSpanProvider.Lock()
	defer serverSpanProvider.Unlock()
	previous := serverSpanProvider.provider
	serverSpanProvider.provider = provider
	return previous
}

// V2TracingServerSpanProvider returns the configured provider, or nil when v2
// server-span interoperability has not been installed.
func V2TracingServerSpanProvider() ServerSpanProvider {
	serverSpanProvider.RLock()
	defer serverSpanProvider.RUnlock()
	return serverSpanProvider.provider
}
