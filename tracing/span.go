package tracing

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// SpanType enum.
type SpanType int

// SpanType values.
const (
	SpanTypeLocal SpanType = iota
	SpanTypeClient
	SpanTypeServer
)

const (
	client  = "client"
	local   = "local"
	server  = "server"
	unknown = "unknown"
)

func (st SpanType) String() string {
	switch st {
	default:
		return unknown
	case SpanTypeServer:
		return server
	case SpanTypeClient:
		return client
	case SpanTypeLocal:
		return local
	}
}

type contextKey int

const (
	serverSpanKey contextKey = iota
	activeSpanKey
)

// CreateServerSpan creates a new server Span, calls any registered
// CreateServerSpanHooks, and starts the Span.
func CreateServerSpan(tracer *Tracer, name string) *Span {
	span := newSpan(tracer, name, SpanTypeServer)
	onCreateServerSpan(span)
	span.onStart()
	return span
}

// CreateServerSpanForContext creates a new server Span, calls any registered
// CreateServerSpanHooks, and starts the Span as well as a Context with the span set as
// the server Span.
func CreateServerSpanForContext(ctx context.Context, tracer *Tracer, name string) (context.Context, *Span) {
	span := CreateServerSpan(tracer, name)
	// It's safe to ignore the error returned by span.SetServerSpan because it
	// only returns an error if span.SpanType != SpanTypeServer which we know it
	// does.
	ctx, _ = span.SetServerSpan(ctx)
	return ctx, span
}

// GetServerSpan gets the server Span from the given context.
func GetServerSpan(ctx context.Context) *Span {
	if span, ok := ctx.Value(serverSpanKey).(*Span); ok {
		return span
	}
	return nil
}

// GetActiveSpan gets the currenty active Span from the given context.
func GetActiveSpan(ctx context.Context) *Span {
	if span, ok := ctx.Value(activeSpanKey).(*Span); ok {
		return span
	}
	return nil
}

func newSpan(tracer *Tracer, name string, spanType SpanType) *Span {
	span := &Span{
		trace:    newTrace(tracer, name),
		spanType: spanType,
	}
	switch spanType {
	case SpanTypeServer:
		span.trace.timeAnnotationReceiveKey = ZipkinTimeAnnotationKeyServerReceive
		span.trace.timeAnnotationSendKey = ZipkinTimeAnnotationKeyServerSend
	case SpanTypeClient:
		span.trace.timeAnnotationReceiveKey = ZipkinTimeAnnotationKeyClientReceive
		span.trace.timeAnnotationSendKey = ZipkinTimeAnnotationKeyClientSend
	}
	return span
}

// Span defines a tracing span.
type Span struct {
	trace *trace

	component string
	hooks     []interface{}
	spanType  SpanType
}

func (s *Span) onStart() {
	for _, h := range s.hooks {
		if hook, ok := h.(StartStopSpanHook); ok {
			if err := hook.OnPostStart(s); err != nil {
				s.LogError("OnPostStart hook error: ", err)
			}
		}
	}
}

// Name returns the name of the Span.
func (s Span) Name() string {
	return s.trace.name
}

// SpanType returns the SpanType for the Span.
func (s Span) SpanType() SpanType {
	return s.spanType
}

// LogError is a helper method to log an error plus a message.
//
// This uses the the Logger provided by the underlying tracing.Tracer used to
// publish the Span.
func (s Span) LogError(msg string, err error) {
	if s.trace.tracer.Logger != nil {
		s.trace.tracer.Logger(msg + err.Error())
	}
}

// AddHooks adds hooks into the Span.
//
// Any hooks that do not conform to at least one of the span hook interfaces
// will be discarded and an error will be logged.
//
// It is recommended that you only call AddHooks on a Span within an
// OnCreate Hook so the Span is set up with all of it's Hooks as a part of
// it's creation.
func (s *Span) AddHooks(hooks ...interface{}) {
	for _, hook := range hooks {
		if IsSpanHook(hook) {
			s.hooks = append(s.hooks, hook)
		} else {
			s.LogError(
				"AddHooks error: ",
				fmt.Errorf(
					"attempting to add non-SpanHook object %T into span's hook registry: %#v",
					hook,
					hook,
				),
			)
		}
	}
}

// SetDebug sets or unsets the debug flag of this Span.
func (s *Span) SetDebug(v bool) {
	s.trace.setDebug(v)
}

// SetTag sets a binary tag annotation and calls all OnSetTag Hooks
// registered to the Span.
func (s *Span) SetTag(key string, value interface{}) {
	s.trace.setTag(key, value)
	for _, h := range s.hooks {
		if hook, ok := h.(SetSpanTagHook); ok {
			if err := hook.OnSetTag(s, key, value); err != nil {
				s.LogError("OnSetTag hook error: ", err)
			}
		}
	}
}

// AddCounter adds delta to a counter annotation and calls all OnAddCounter
// Hooks registered to the Span.
func (s *Span) AddCounter(key string, delta float64) {
	s.trace.addCounter(key, delta)
	for _, h := range s.hooks {
		if hook, ok := h.(AddSpanCounterHook); ok {
			if err := hook.OnAddCounter(s, key, delta); err != nil {
				s.LogError("OnAddCounter hook error: ", err)
			}
		}
	}
}

// CreateClientChild creates a SpanTypeClient Span with given name that is a
// child of the Span, starts it, and calls all OnCreateChildHooks registered to
// the parent Span.
//
// A client child-span should be used to make requests to other upstream
// servers.
//
// Timestamps, counters, and tags won't be inherited.
// Parent id will be inherited from the span id,
// and span id will be randomly generated.
// Trace id, sampled, and flags will be copied over.
func (s Span) CreateClientChild(name string) *Span {
	child := newSpan(s.trace.tracer, name, SpanTypeClient)
	s.initChildSpan(child)
	return child
}

// CreateClientChildForContext creates a client Span with given name that is a
// child of the Span, starts it, and calls all OnCreateChildHooks registered to
// the parent Span as well as a Context that can be used by the client you
// are creating the span for.
//
// A client child-span should be used to make requests to other upstream
// servers.
//
// Timestamps, counters, and tags won't be inherited.
// Parent id will be inherited from the span id,
// and span id will be randomly generated.
// Trace id, sampled, and flags will be copied over.
func (s Span) CreateClientChildForContext(ctx context.Context, name string) (context.Context, *Span) {
	child := s.CreateClientChild(name)
	return context.WithValue(ctx, activeSpanKey, child), child
}

// ChildAndThriftContext creates both a client child span and a context can be
// used by the thrift client code.
//
// A thrift client call would look like:
//
//     clientCtx, span := parentSpan.ChildAndThriftContext(ctx, "myCall")
//     result, err := client.MyCall(clientCtx, arg1, arg2)
//     span.End(ctx, err)
func (s Span) ChildAndThriftContext(ctx context.Context, name string) (context.Context, *Span) {
	ctx, child := s.CreateClientChildForContext(ctx, name)
	ctx = CreateThriftContextFromSpan(ctx, child)
	return ctx, child
}

// CreateLocalChild creates a SpanTypeLocal Span with given name and component
// that is a child of the Span, starts it, and calls all OnCreateChildHooks
// registered to the parent Span.
//
// component is an optional, local component name and may be empty string.
//
// Timestamps, counters, and tags won't be inherited.
// Parent id will be inherited from the span id,
// and span id will be randomly generated.
// Trace id, sampled, and flags will be copied over.
func (s Span) CreateLocalChild(name, component string) *Span {
	child := newSpan(s.trace.tracer, name, SpanTypeLocal)
	child.component = component
	s.initChildSpan(child)
	return child
}

// CreateLocalChildForContext creates a local Span with given name and component
// that is a child of the Span, starts it, and calls all OnCreateChildHooks
// registered to the parent Span as well as a Context that can be used by
// the client you are creating the span for.
//
// component is an optional, local component name and may be empty string.
//
// Timestamps, counters, and tags won't be inherited.
// Parent id will be inherited from the span id,
// and span id will be randomly generated.
// Trace id, sampled, and flags will be copied over.
func (s Span) CreateLocalChildForContext(ctx context.Context, name, component string) (context.Context, *Span) {
	child := s.CreateLocalChild(name, component)
	return context.WithValue(ctx, activeSpanKey, child), child
}

func (s Span) initChildSpan(child *Span) {
	child.trace.parentID = s.trace.spanID
	child.trace.traceID = s.trace.traceID
	child.trace.sampled = s.trace.sampled
	child.trace.flags = s.trace.flags
	for _, h := range s.hooks {
		if hook, ok := h.(CreateChildSpanHook); ok {
			if err := hook.OnCreateChild(&s, child); err != nil {
				s.LogError("OnCreateChild hook error: ", err)
			}
		}
	}
	child.onStart()
}

// SetServerSpan sets the Span as the ServerSpan on the given context
func (s *Span) SetServerSpan(ctx context.Context) (context.Context, error) {
	if s.SpanType() != SpanTypeServer {
		return ctx, &InvalidSpanTypeError{
			ExpectedSpanType: SpanTypeServer,
			ActualSpanType:   s.SpanType(),
		}
	}
	return context.WithValue(ctx, serverSpanKey, s), nil
}

// Stop stops the Span, calls all registered OnPreStop Hooks, serializes the Span,
// and sends the serialized Span to a back-end that records the Span.
func (s *Span) Stop(ctx context.Context, err error) error {
	s.preStop(err)
	for _, h := range s.hooks {
		if hook, ok := h.(StartStopSpanHook); ok {
			if hookErr := hook.OnPreStop(s, err); hookErr != nil {
				s.LogError("OnPreStop hook error: ", hookErr)
			}
		}
	}
	s.trace.stop = time.Now()
	return s.trace.publish(ctx)
}

func (s *Span) preStop(err error) {
	// We intentionally don't use the top level span.SetTag function
	// because we don't want to trigger any OnSetTag Hooks in this case.
	switch s.spanType {
	case SpanTypeServer:
		if err != nil && errors.Is(err, context.DeadlineExceeded) {
			s.trace.setTag(ZipkinBinaryAnnotationKeyTimeOut, true)
		}
	case SpanTypeLocal:
		if s.component != "" {
			s.trace.setTag(ZipkinBinaryAnnotationKeyLocalComponent, s.component)
		}
	}
	if err != nil {
		s.trace.setTag(ZipkinBinaryAnnotationKeyError, true)
	}
	if s.trace.isDebugSet() {
		s.trace.setTag(ZipkinBinaryAnnotationKeyDebug, true)
	}
}
