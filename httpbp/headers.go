package httpbp

import (
	"context"
	"net/http"
)

const (
	// EdgeContextHeader is the key use to get the raw edge context from
	// the HTTP request headers.
	EdgeContextHeader = "X-Edge-Request"

	// ParentIDHeader is the key use to get the span parent ID from
	// the HTTP request headers.
	ParentIDHeader = "X-Parent"

	// SpanIDHeader is the key use to get the span ID from the HTTP
	// request headers.
	SpanIDHeader = "X-Span"

	// SpanFlagsHeader is the key use to get the span flags from the HTTP
	// request headers.
	SpanFlagsHeader = "X-Flags"

	// SpanSampledHeader is the key use to get the sampled flag from the
	// HTTP request headers.
	SpanSampledHeader = "X-Sampled"

	// TraceIDHeader is the key use to get the trace ID from the HTTP
	// request headers.
	TraceIDHeader = "X-Trace"
)

type headerContextKey int

const (
	edgeContextContextKey headerContextKey = iota
	traceIDContextKey
	parentIDContextKey
	spanIDContextKey
	spanFlagsContextKey
	spanSampledContextKey
)

// HeaderContextKey is an Enum used to get HTTP headers from a context object.
//
// It is not used as the actual key in the context, these enums are mapped to
// un-exported values that serve as the actual context keys.
type HeaderContextKey int

const (
	// EdgeContextContextKey is the key for the raw edge request context
	EdgeContextContextKey HeaderContextKey = iota

	// TraceIDContextKey is the header for the trace ID passed by the caller
	TraceIDContextKey

	// ParentIDContextKey is the header for the parent ID passed by the caller
	ParentIDContextKey

	// SpanIDContextKey is the header for the span ID passed by the caller
	SpanIDContextKey

	// SpanFlagsContextKey is the header for the span flags passed by the caller
	SpanFlagsContextKey

	// SpanSampledContextKey is the header for the sampled flag passed by the caller
	SpanSampledContextKey
)

var headerKeyToContextKey = map[HeaderContextKey]headerContextKey{
	EdgeContextContextKey: edgeContextContextKey,
	TraceIDContextKey:     traceIDContextKey,
	ParentIDContextKey:    parentIDContextKey,
	SpanIDContextKey:      spanIDContextKey,
	SpanFlagsContextKey:   spanFlagsContextKey,
	SpanSampledContextKey: spanSampledContextKey,
}

// GetHeader returns the HTTP header stored on the context at key.
func GetHeader(ctx context.Context, key HeaderContextKey) (header string) {
	if contextKey, ok := headerKeyToContextKey[key]; ok {
		if h, ok := ctx.Value(contextKey).(string); ok {
			header = h
		}
	}
	return
}

// Headers is an interface to collect all of the HTTP headers for a particular
// baseplate resource (spans and edge contexts) into a struct that provides an
// easy way to convert them into HTTP headers.
//
// This interface exists so we can avoid having to do runtime checks on maps to
// ensure that they have the right keys set when we are trying to sign or verify
// a set of HTTP headers.
type Headers interface {
	// AsMap returns the Headers struct as a map of header keys to header
	// values.
	AsMap() map[string]string
}

// EdgeContextHeaders implements the Headers interface for HTTP EdgeContext
// headers.
type EdgeContextHeaders struct {
	EdgeRequest string
}

// NewEdgeContextHeaders returns a new EdgeContextHeaders object from the given
// HTTP headers.
func NewEdgeContextHeaders(h http.Header) *EdgeContextHeaders {
	return &EdgeContextHeaders{
		EdgeRequest: h.Get(EdgeContextHeader),
	}
}

// AsMap returns the EdgeContextHeaders as a map of header keys to header
// values.
func (s *EdgeContextHeaders) AsMap() map[string]string {
	return map[string]string{
		EdgeContextHeader: s.EdgeRequest,
	}
}

// SpanHeaders implements the Headers interface for HTTP Span headers.
type SpanHeaders struct {
	TraceID  string
	ParentID string
	SpanID   string
	Flags    string
	Sampled  string
}

// NewSpanHeaders returns a new SpanHeaders object from the given HTTP headers.
func NewSpanHeaders(h http.Header) *SpanHeaders {
	return &SpanHeaders{
		TraceID:  h.Get(TraceIDHeader),
		ParentID: h.Get(ParentIDHeader),
		SpanID:   h.Get(SpanIDHeader),
		Flags:    h.Get(SpanFlagsHeader),
		Sampled:  h.Get(SpanSampledHeader),
	}
}

// AsMap returns the SpanHeaders as a map of header keys to header values.
func (s *SpanHeaders) AsMap() map[string]string {
	return map[string]string{
		TraceIDHeader:     s.TraceID,
		ParentIDHeader:    s.ParentID,
		SpanIDHeader:      s.SpanID,
		SpanFlagsHeader:   s.Flags,
		SpanSampledHeader: s.Sampled,
	}
}

var (
	_ Headers = (*EdgeContextHeaders)(nil)
	_ Headers = (*SpanHeaders)(nil)
)

// InjectBaseplateContextFromHTTP takes baseplate HTTP headers from the request,
// verifies that it should trust the headers using the provided
// HeaderTrustHandler, and attaches the trusted headers to the context.
//
// These headers can be retrieved using httpbp.GetHeader.
func InjectBaseplateContextFromHTTP(ctx context.Context, t HeaderTrustHandler, r *http.Request) context.Context {
	if t.TrustEdgeContext(r) {
		ctx = context.WithValue(
			ctx,
			edgeContextContextKey,
			r.Header.Get(EdgeContextHeader),
		)
	}

	if t.TrustSpan(r) {
		for k, v := range map[headerContextKey]string{
			traceIDContextKey:     r.Header.Get(TraceIDHeader),
			parentIDContextKey:    r.Header.Get(ParentIDHeader),
			spanIDContextKey:      r.Header.Get(SpanIDHeader),
			spanFlagsContextKey:   r.Header.Get(SpanFlagsHeader),
			spanSampledContextKey: r.Header.Get(SpanSampledHeader),
		} {
			ctx = context.WithValue(ctx, k, v)
		}
	}

	return ctx
}

// PopulateBaseplateRequestContext returns a function that calls
// InjectBaseplateContextFromHTTP with the HeaderTrustHandler you pass to it.
// The function that this produces can be passed to go-kit's http.ServerBefore
// ServerOption.
//
// example:
//		trustHandler := httpbp.NeverTrustHeaders{}
//		httpgk.NewServer(
//			endpoints.IsHealthy,
//			DecodeIsHealthyRequest,
//			EncodeResponse,
//			httpgk.ServerBefore(
//				httpbp.PopulateRequestContext(trustHandler),
//			),
//		)
func PopulateBaseplateRequestContext(t HeaderTrustHandler) func(ctx context.Context, r *http.Request) context.Context {
	return func(ctx context.Context, r *http.Request) context.Context {
		return InjectBaseplateContextFromHTTP(ctx, t, r)
	}
}
