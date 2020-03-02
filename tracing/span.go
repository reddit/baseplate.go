package tracing

import (
	"context"
	"errors"
	"fmt"
	"time"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
)

var (
	_ opentracing.SpanContext = (*Span)(nil)
	_ opentracing.Span        = (*Span)(nil)
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

// AsSpan converts an opentracing.Span back to *Span.
//
// This function never returns nil.
// If the passed in opentracing.Span is actually not implemented by *Span,
// a new *Span with empty name and local type will be created and returned.
// When that happens it will also be logged if the last InitGlobalTracer call
// was with a non-nil logger.
//
// This function is provided for convenience calling functions not in
// opentracing Span API, for example:
//
//     span := opentracing.StartSpan(name, opts...)
//     tracing.AsSpan(span).AddHooks(hooks...)
func AsSpan(s opentracing.Span) *Span {
	if span, ok := s.(*Span); ok && span != nil {
		return span
	}
	globalTracer.getLogger()(fmt.Sprintf(
		"Failed to cast opentracing.Span %#v back to *tracing.Span.",
		s,
	))
	return newSpan(nil, "", SpanTypeLocal)
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
// This uses the the logger provided by the underlying tracing.Tracer used to
// publish the Span.
func (s Span) LogError(msg string, err error) {
	s.trace.tracer.getLogger()(msg + err.Error())
}

// AddHooks adds hooks into the Span.
//
// Any hooks that do not conform to at least one of the span hook interfaces
// will be discarded and an error will be logged.
//
// It is recommended that you only call AddHooks on a Span within an
// OnCreateChild/OnCreateServerSpan hook so the Span is set up with all of its
// hooks as a part of its creation.
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
func (s *Span) SetTag(key string, value interface{}) opentracing.Span {
	s.trace.setTag(key, value)
	for _, h := range s.hooks {
		if hook, ok := h.(SetSpanTagHook); ok {
			if err := hook.OnSetTag(s, key, value); err != nil {
				s.LogError("OnSetTag hook error: ", err)
			}
		}
	}
	return s
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

// Component returns the local component name of this span, with special cases.
//
// For local spans,
// this returns the component name set while starting the span,
// or "local" if it's empty.
// For client spans, this returns "clients".
// For all other span types, this returns the string version of the span type.
func (s *Span) Component() string {
	switch s.spanType {
	case SpanTypeClient:
		return "clients"
	case SpanTypeLocal:
		if s.component != "" {
			return s.component
		}
	}
	return s.spanType.String()
}

func (s Span) initChildSpan(child *Span) {
	child.trace.parentID = s.trace.spanID
	child.trace.traceID = s.trace.traceID
	child.trace.sampled = s.trace.sampled
	child.trace.flags = s.trace.flags

	if child.spanType != SpanTypeServer {
		// We treat server spans differently. They should only be child to a span
		// from the client side, and have their own create hooks, so we don't call
		// their hooks here. See also: Tracer.StartSpan.
		for _, h := range s.hooks {
			if hook, ok := h.(CreateChildSpanHook); ok {
				if err := hook.OnCreateChild(&s, child); err != nil {
					s.LogError("OnCreateChild hook error: ", err)
				}
			}
		}
		child.onStart()
	}
}

// Stop stops the Span, calls all registered OnPreStop Hooks,
// serializes the Span,
// and sends the serialized Span to a back-end that records the Span.
//
// In most cases FinishWithOptions should be used instead,
// which calls Stop and auto logs the error returned by Stop.
// Stop is still provided in case there's need to handle the error differently.
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

// ForeachBaggageItem implements opentracing.SpanContext.
//
// We don't support any extra baggage items, so it's a noop.
func (s *Span) ForeachBaggageItem(handler func(k, v string) bool) {}

// SetBaggageItem implements opentracing.Span.
//
// As we don't support any extra baggage items,
// it's a noop and just returns self.
func (s *Span) SetBaggageItem(restrictedKey, value string) opentracing.Span {
	return s
}

// BaggageItem implements opentracing.Span.
//
// As we don't support any extra baggage items, it always returns empty string.
func (s *Span) BaggageItem(restrictedKey string) string {
	return ""
}

// Finish implements opentracing.Span.
//
// It calls Stop with background context and nil error.
// If Stop returns an error, it will also be logged with the tracer's logger.
func (s *Span) Finish() {
	if err := s.Stop(context.Background(), nil); err != nil {
		s.LogError("Span.Stop returned error: ", err)
	}
}

// FinishWithOptions implements opentracing.Span.
//
// In this implementation we ignore all timestamps in opts,
// only extract context and error out of all the log fields,
// and ignore all other log fields.
//
// Please use FinishOptions.Convert() to prepare the opts arg.
//
// It calls Stop with context and error extracted from opts.
// If Stop returns an error, it will also be logged with the tracer's logger.
func (s *Span) FinishWithOptions(opts opentracing.FinishOptions) {
	var err error
	ctx := context.Background()
	for _, records := range opts.LogRecords {
		for _, field := range records.Fields {
			switch field.Key() {
			case ctxKey:
				if c, ok := field.Value().(context.Context); ok {
					ctx = c
				}
			case errorKey:
				if e, ok := field.Value().(error); ok {
					err = e
				}
			}
		}
	}
	if stopErr := s.Stop(ctx, err); stopErr != nil {
		s.LogError("Span.Stop returned error: ", stopErr)
	}
}

// Context implements opentracing.Span.
//
// It returns self as opentracing.SpanContext.
func (s *Span) Context() opentracing.SpanContext {
	return s
}

// SetOperationName implements opentracing.Span.
func (s *Span) SetOperationName(operationName string) opentracing.Span {
	s.trace.name = operationName
	return s
}

// Tracer implements opentracing.Span.
func (s *Span) Tracer() opentracing.Tracer {
	return s.trace.tracer
}

// LogFields implements opentracing.Span.
//
// In this implementation it's a no-op.
func (s *Span) LogFields(fields ...log.Field) {}

// LogKV implements opentracing.Span.
//
// In this implementation it's a no-op.
func (s *Span) LogKV(alternatingKeyValues ...interface{}) {}

// LogEvent implements opentracing.Span.
//
// it's deprecated in the interface and is a no-op here.
func (s *Span) LogEvent(event string) {}

// LogEventWithPayload implements opentracing.Span.
//
// it's deprecated in the interface and is a noop here.
func (s *Span) LogEventWithPayload(event string, payload interface{}) {}

// Log implements opentracing.Span.
//
// it's deprecated in the interface and is a no-op here.
func (s *Span) Log(data opentracing.LogData) {}
