package thriftbp

import (
	"context"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/reddit/baseplate.go/ecinterface"
	"github.com/reddit/baseplate.go/transport"
)

// Edge request context propagation related headers, as defined in
// https://pages.github.snooguts.net/reddit/baseplate.spec/component-apis/thrift#edge-request-context-propagation
const (
	HeaderEdgeRequest = transport.HeaderEdgeRequest
)

// Tracing related headers, as defined in
// https://pages.github.snooguts.net/reddit/baseplate.spec/component-apis/thrift#tracing
const (
	// The Trace ID, a 64-bit integer encoded in decimal.
	HeaderTracingTrace = transport.HeaderTracingTrace
	// The Span ID, a 64-bit integer encoded in decimal.
	HeaderTracingSpan = transport.HeaderTracingSpan
	// The Parent Span ID, a 64-bit integer encoded in decimal.
	HeaderTracingParent = transport.HeaderTracingParent
	// The Sampled flag, an ASCII "1" (HeaderTracingSampledTrue) if true,
	// otherwise false.
	// If not present, defaults to false.
	HeaderTracingSampled = transport.HeaderTracingSampled
	// Trace flags, a 64-bit integer encoded in decimal.
	// If not present, defaults to null.
	HeaderTracingFlags = transport.HeaderTracingFlags
)

// HeaderTracingSampledTrue is the header value to indicate that this trace
// should be sampled.
const HeaderTracingSampledTrue = transport.HeaderTracingSampledTrue

// Deadline propagation related headers.
const (
	// Number of milliseconds, 64-bit integer encoded in decimal.
	HeaderDeadlineBudget = transport.HeaderDeadlineBudget
)

// UserAgent related headers.
const (
	HeaderUserAgent = "User-Agent"
)

// HeadersToForward are the headers that should always be forwarded to upstream
// thrift servers, to be used in thrift.TSimpleServer.SetForwardHeaders.
var HeadersToForward = []string{
	HeaderEdgeRequest,
	HeaderTracingTrace,
	HeaderTracingSpan,
	HeaderTracingParent,
	HeaderTracingSampled,
	HeaderTracingFlags,
}

// AttachEdgeRequestContext returns a context that has the header of the edge
// context attached to ctx object set to forward using the "Edge-Request" header
// on any Thrift calls made with that context object.
func AttachEdgeRequestContext(ctx context.Context, ecImpl ecinterface.Interface) context.Context {
	if ecImpl == nil {
		ecImpl = ecinterface.Get()
	}
	header, ok := ecImpl.ContextToHeader(ctx)
	if !ok {
		return thrift.UnsetHeader(ctx, HeaderEdgeRequest)
	}
	return AddClientHeader(ctx, HeaderEdgeRequest, header)
}

// AddClientHeader adds a key-value pair to thrift client's headers.
//
// It takes care of setting the header in context (overwrite previous value if
// any), and also adding the header to the write header list.
func AddClientHeader(ctx context.Context, key, value string) context.Context {
	headers := thrift.GetWriteHeaderList(ctx)
	ctx = thrift.SetHeader(ctx, key, value)
	headers = append(headers, key)
	return thrift.SetWriteHeaderList(ctx, headers)
}
