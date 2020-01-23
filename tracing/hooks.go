package tracing

// BaseplateHook allows you to inject functionality into the lifecycle of a
// Baseplate request.
type BaseplateHook interface {
	OnServerSpanCreate(span *Span) error
}

// SpanHook allows you to inject functionality into the lifecycle of Baseplate
// Spans.
type SpanHook interface {
	OnCreateChild(child *Span) error
	OnStart(span *Span) error
	OnEnd(span *Span, err error) error
	OnSetTag(span *Span, key string, value interface{}) error
	OnAddCounter(span *Span, key string, delta float64) error
}

var (
	hooks []BaseplateHook
)

// RegisterBaseplateHook registers a Hook into the Baseplate request lifecycle.
//
// This function and ResetHooks are not safe to call concurrently.
func RegisterBaseplateHook(hook BaseplateHook) {
	hooks = append(hooks, hook)
}

// ResetHooks removes all global hooks and resets back to initial state.
//
// This function and RegisterBaseplateHook are not safe to call concurrently.
func ResetHooks() {
	hooks = nil
}

func onServerSpanCreate(span *Span) {
	for _, hook := range hooks {
		if err := hook.OnServerSpanCreate(span); err != nil && span.tracer.Logger != nil {
			span.tracer.Logger("OnCreateServerSpan hook error: " + err.Error())
		}
	}
}

// NopSpanHook can be embedded in a SpanHook implementation to provide default,
// no-op implementations for all methods in the SpanHook interface.
type NopSpanHook struct{}

// OnCreateChild is a nop
func (h NopSpanHook) OnCreateChild(child *Span) error {
	return nil
}

// OnStart is a nop
func (h NopSpanHook) OnStart(span *Span) error {
	return nil
}

// OnEnd is a nop
func (h NopSpanHook) OnEnd(span *Span, err error) error {
	return nil
}

// OnSetTag is a nop
func (h NopSpanHook) OnSetTag(span *Span, key string, value interface{}) error {
	return nil
}

// OnAddCounter is a nop
func (h NopSpanHook) OnAddCounter(span *Span, key string, delta float64) error {
	return nil
}

var _ SpanHook = NopSpanHook{}
