package tracing

import (
	"context"
)

// CreateServerSpanHook allows you to inject functionality into the lifecycle of a
// Baseplate request.
type CreateServerSpanHook interface {
	// OnCreateServerSpan is called after a server Span is first created by
	// tracing.CreateServerSpan, before any OnPostStart Hooks are called.
	//
	// OnCreateServerSpan is the recommended place to register Hooks onto the
	// server Span.
	OnCreateServerSpan(span *Span) error
}

// CreateChildSpanHook allows you to inject functionality into the creation of a
// Baseplate span.
type CreateChildSpanHook interface {
	// OnCreateChild is called after a child Span is first created, before any
	// OnPostStart Hooks are called.
	//
	// OnCreateChild is the recommended place to register Hooks onto the
	// child Span.
	OnCreateChild(parent, child *Span) error
}

// StartStopSpanHook allows you to inject functionality immediately after
// starting a span and immediately before stopping a span.
type StartStopSpanHook interface {
	// OnPostStart is called after a child Span is created and the OnCreateChild
	// Hooks on the parent Span are called.
	OnPostStart(span *Span) error

	// OnPreStop is called by Span.Stop, after setting any custom tags, but
	// before the span is stopped, serialized, or published.
	OnPreStop(span *Span, err error) error
}

// SetSpanTagHook allows you to inject functionality after setting a tag on a
// span.
type SetSpanTagHook interface {
	// OnSetTag is called by Span.SetTag, after the tag is set on the Span.
	OnSetTag(span *Span, key string, value interface{}) error
}

// AddSpanCounterHook allows you to inject functionality after adding a counter
// on a span.
type AddSpanCounterHook interface {
	// OnAddCounter is called by Span.AddCounter, after the counter is updated
	// on the Span.
	OnAddCounter(span *Span, key string, delta float64) error
}

var (
	createServerSpanHooks []CreateServerSpanHook
)

// IsSpanHook returns true if hook implements at least one of the span Hook
// interfaces and false if it implements none.
func IsSpanHook(hook interface{}) bool {
	if _, ok := hook.(CreateChildSpanHook); ok {
		return ok
	}
	if _, ok := hook.(StartStopSpanHook); ok {
		return ok
	}
	if _, ok := hook.(SetSpanTagHook); ok {
		return ok
	}
	if _, ok := hook.(AddSpanCounterHook); ok {
		return ok
	}
	return false
}

// RegisterCreateServerSpanHooks registers Hooks into the Baseplate request
// lifecycle.
//
// This function and ResetHooks are not safe to call concurrently.
func RegisterCreateServerSpanHooks(hooks ...CreateServerSpanHook) {
	createServerSpanHooks = append(createServerSpanHooks, hooks...)
}

// ResetHooks removes all global hooks and resets back to initial state.
//
// This function and RegisterCreateServerSpanHooks are not safe to call concurrently.
func ResetHooks() {
	createServerSpanHooks = nil
}

func onCreateServerSpan(span *Span) {
	if span.SpanType() != SpanTypeServer {
		span.logError(
			context.Background(),
			"OnCreateServerSpan called on non-server Span: ",
			&InvalidSpanTypeError{
				ExpectedSpanType: SpanTypeServer,
				ActualSpanType:   span.SpanType(),
			},
		)
		return
	}

	for _, hook := range createServerSpanHooks {
		if err := hook.OnCreateServerSpan(span); err != nil {
			span.logError(context.Background(), "OnCreateServerSpan hook error: ", err)
		}
	}
}
