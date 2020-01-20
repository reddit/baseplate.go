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
}

var (
	hooks []BaseplateHook
)

// RegisterBaseplateHook registers a Hook into the Baseplate request lifecycle.
func RegisterBaseplateHook(hook BaseplateHook) {
	hooks = append(hooks, hook)
}

// ResetHooks removes all global hooks and resets back to initial state.
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
