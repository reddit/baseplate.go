package tracing

import (
	"context"
	"math/rand"
	"time"

	"github.com/reddit/baseplate.go/timebp"
)

// SpanType enum.
type SpanType int

// SpanType values.
const (
	SpanTypeLocal SpanType = iota
	SpanTypeClient
	SpanTypeServer
)

type contextKey int

const (
	serverSpanKey contextKey = iota
	childSpanKey
)

// FlagMask values.
//
// Reference: https://github.com/reddit/baseplate.py/blob/1ca8488bcd42c8786e6a3db35b2a99517fd07a99/baseplate/observers/tracing.py#L60-L64
const (
	// When set, this trace passes all samplers.
	FlagMaskDebug int64 = 1
)

const (
	counterKeyPrefix   = "counter."
	baseplateComponent = "baseplate"
)

// Span defines a tracing span.
type Span struct {
	Name string

	tracer *Tracer

	traceID  uint64
	spanID   uint64
	parentID uint64
	sampled  bool

	component string

	start    time.Time
	end      time.Time
	spanType SpanType
	flags    int64

	counters map[string]float64
	tags     map[string]interface{}
	hooks    []SpanHook
}

func newSpan(tracer *Tracer, name string, spanType SpanType) *Span {
	if tracer == nil {
		tracer = &GlobalTracer
	}
	span := &Span{
		Name: name,

		tracer:   tracer,
		traceID:  rand.Uint64(),
		spanID:   rand.Uint64(),
		start:    time.Now(),
		spanType: spanType,

		counters: make(map[string]float64),
		tags: map[string]interface{}{
			ZipkinBinaryAnnotationKeyComponent: baseplateComponent,
		},
	}
	return span
}

// CreateServerSpan creates a new Server Span, calls any registered
// BaseplateHooks, and starts the Span.
func CreateServerSpan(tracer *Tracer, name string) *Span {
	span := newSpan(tracer, name, SpanTypeServer)
	onServerSpanCreate(span)
	span.startSpan()
	return span
}

func (s *Span) startSpan() {
	for _, hook := range s.hooks {
		if err := hook.OnStart(s); err != nil && s.tracer.Logger != nil {
			s.tracer.Logger("OnStart hook error: " + err.Error())
		}
	}
}

// RegisterHook adds a SpanHook into the spans registry of hooks to run.
func (s *Span) RegisterHook(hook SpanHook) {
	s.hooks = append(s.hooks, hook)
}

// ToZipkinSpan returns a ZipkinSpan with data copied from this span.
//
// If it's called before End is called,
// Duration and some annotations might be incorrect/incomplete.
func (s *Span) ToZipkinSpan() ZipkinSpan {
	zs := ZipkinSpan{
		TraceID:  s.traceID,
		Name:     s.Name,
		SpanID:   s.spanID,
		Start:    timebp.TimestampMicrosecond(s.start),
		ParentID: s.parentID,
	}
	end := s.end
	if end.IsZero() {
		end = time.Now()
	}
	zs.Duration = timebp.DurationMicrosecond(end.Sub(s.start))

	var endpoint ZipkinEndpointInfo
	if s.tracer != nil {
		endpoint = s.tracer.Endpoint
	}

	switch s.spanType {
	case SpanTypeServer:
		zs.TimeAnnotations = []ZipkinTimeAnnotation{
			{
				Endpoint:  endpoint,
				Key:       ZipkinTimeAnnotationKeyServerReceive,
				Timestamp: timebp.TimestampMicrosecond(s.start),
			},
			{
				Endpoint:  endpoint,
				Key:       ZipkinTimeAnnotationKeyServerSend,
				Timestamp: timebp.TimestampMicrosecond(end),
			},
		}
	case SpanTypeClient:
		zs.TimeAnnotations = []ZipkinTimeAnnotation{
			{
				Endpoint:  endpoint,
				Key:       ZipkinTimeAnnotationKeyClientSend,
				Timestamp: timebp.TimestampMicrosecond(s.start),
			},
			{
				Endpoint:  endpoint,
				Key:       ZipkinTimeAnnotationKeyClientReceive,
				Timestamp: timebp.TimestampMicrosecond(end),
			},
		}
	}

	zs.BinaryAnnotations = make([]ZipkinBinaryAnnotation, 0, len(s.counters)+len(s.tags))
	for key, value := range s.counters {
		zs.BinaryAnnotations = append(
			zs.BinaryAnnotations,
			ZipkinBinaryAnnotation{
				Endpoint: endpoint,
				Key:      counterKeyPrefix + key,
				Value:    value,
			},
		)
	}
	for key, value := range s.tags {
		zs.BinaryAnnotations = append(
			zs.BinaryAnnotations,
			ZipkinBinaryAnnotation{
				Endpoint: endpoint,
				Key:      key,
				Value:    value,
			},
		)
	}

	return zs
}

func (s *Span) isDebugSet() bool {
	return s.flags&FlagMaskDebug != 0
}

// ShouldSample returns true if this span should be sampled.
//
// If the span's debug flag was set, then this function will always return true.
func (s *Span) ShouldSample() bool {
	return s.sampled || s.isDebugSet()
}

// End ends a span.
//
// If err is non-nil, the error tag will also be set to true.
//
// It also sends the span into tracer, if the span should be sampled.
// The context object passed in is used to control the timeout of the sending,
// see Tracer.MaxRecordTimeout for more info.
func (s *Span) End(ctx context.Context, err error) error {
	s.end = time.Now()

	if err != nil {
		s.SetTag(ZipkinBinaryAnnotationKeyError, true)
	}
	if s.isDebugSet() {
		s.SetTag(ZipkinBinaryAnnotationKeyDebug, true)
	}
	if s.spanType == SpanTypeLocal && s.component != "" {
		s.SetTag(ZipkinBinaryAnnotationKeyLocalComponent, s.component)
	}
	for _, hook := range s.hooks {
		if hookErr := hook.OnEnd(s, err); hookErr != nil && s.tracer.Logger != nil {
			s.tracer.Logger("OnEnd hook error: " + hookErr.Error())
		}
	}
	if s.tracer != nil {
		return s.tracer.Record(ctx, s)
	}
	return nil
}

// AddCounter adds delta to a counter annotation.
func (s *Span) AddCounter(key string, delta float64) {
	s.counters[key] += delta
}

// SetTag sets a binary tag annotation.
func (s *Span) SetTag(key string, value interface{}) {
	s.tags[key] = value
}

// SetDebug sets or unsets the debug flag of this span.
func (s *Span) SetDebug(v bool) {
	if v {
		s.flags |= FlagMaskDebug
	} else {
		s.flags &= ^FlagMaskDebug
	}
}

// CreateLocalChild creates a local child-span with given name and component.
//
// component is optional local component name and could be empty string.
//
// Timestamps, counters, and tags won't be inherited.
// Parent id will be inherited from the span id,
// and span id will be randomly generated.
// Trace id, sampled, and flags will be copied over.
func (s *Span) CreateLocalChild(name string, component string) *Span {
	return s.createChild(name, SpanTypeLocal, component)
}

// CreateLocalChildForContext creates a local child-span with given name and
// component and a context that can be used by the client you are creating the
// span for.
//
// component is optional local component name and could be empty string.
//
// Timestamps, counters, and tags won't be inherited.
// Parent id will be inherited from the span id,
// and span id will be randomly generated.
// Trace id, sampled, and flags will be copied over.
func (s *Span) CreateLocalChildForContext(ctx context.Context, name string, component string) (context.Context, *Span) {
	child := s.CreateLocalChild(name, component)
	return context.WithValue(ctx, childSpanKey, child), child
}

// CreateClientChild creates a client child-span with given name.
//
// A client child-span should be used to make requests to other upstream
// servers.
//
// Timestamps, counters, and tags won't be inherited.
// Parent id will be inherited from the span id,
// and span id will be randomly generated.
// Trace id, sampled, and flags will be copied over.
func (s *Span) CreateClientChild(name string) *Span {
	return s.createChild(name, SpanTypeClient, "")
}

// CreateClientChildForContext creates a client child-span with given name and a
// context that can be used by the client you are creating the span for.
//
// A client child-span should be used to make requests to other upstream
// servers.
//
// Timestamps, counters, and tags won't be inherited.
// Parent id will be inherited from the span id,
// and span id will be randomly generated.
// Trace id, sampled, and flags will be copied over.
func (s *Span) CreateClientChildForContext(ctx context.Context, name string) (context.Context, *Span) {
	child := s.CreateClientChild(name)
	return context.WithValue(ctx, childSpanKey, child), child
}

func (s *Span) createChild(name string, spanType SpanType, component string) *Span {
	span := newSpan(s.tracer, name, spanType)
	span.spanType = spanType
	span.component = component

	span.parentID = s.spanID
	span.traceID = s.traceID
	span.sampled = s.sampled
	span.flags = s.flags

	for _, hook := range s.hooks {
		if err := hook.OnCreateChild(span); err != nil && s.tracer.Logger != nil {
			s.tracer.Logger("OnCreateChild hook error: " + err.Error())
		}
	}
	span.startSpan()
	return span
}

// ChildAndThriftContext creates both a client child span and a context can be
// used by the thrift client code.
//
// A thrift client call would look like:
//
//     clientCtx, span := parentSpan.ChildAndThriftContext(ctx, "myCall")
//     result, err := client.MyCall(clientCtx, arg1, arg2)
//     span.End(ctx, err)
func (s *Span) ChildAndThriftContext(ctx context.Context, name string) (context.Context, *Span) {
	ctx, span := s.CreateClientChildForContext(ctx, name)
	ctx = CreateThriftContextFromSpan(ctx, span)
	return ctx, span
}

// SetServerSpan sets the span as the ServerSpan on the given context
func (s *Span) SetServerSpan(ctx context.Context) (context.Context, error) {
	if s.spanType != SpanTypeServer {
		return nil, &InvalidSpanTypeError{SpanTypeServer, s.spanType}
	}
	return context.WithValue(ctx, serverSpanKey, s), nil
}

func getSpanFromContext(ctx context.Context, key contextKey) *Span {
	if s := ctx.Value(key); s != nil {
		if span, ok := s.(*Span); ok {
			return span
		}
	}
	return nil
}

// GetServerSpan gets the ServerSpan from the given context
func GetServerSpan(ctx context.Context) *Span {
	return getSpanFromContext(ctx, serverSpanKey)
}

// GetChildSpan gets the ChildSpan from the given context
func GetChildSpan(ctx context.Context) *Span {
	return getSpanFromContext(ctx, childSpanKey)
}
