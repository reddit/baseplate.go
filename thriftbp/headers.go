package thriftbp

import (
	"context"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/reddit/baseplate.go/ecinterface"
)

// Edge request context propagation related headers, as defined in
// https://pages.github.snooguts.net/reddit/baseplate.spec/component-apis/thrift#edge-request-context-propagation
const (
	HeaderEdgeRequest = "Edge-Request"
)

// Tracing related headers, as defined in
// https://pages.github.snooguts.net/reddit/baseplate.spec/component-apis/thrift#tracing
const (
	// The Trace ID, a 64-bit integer encoded in decimal.
	HeaderTracingTrace = "Trace"
	// The Span ID, a 64-bit integer encoded in decimal.
	HeaderTracingSpan = "Span"
	// The Parent Span ID, a 64-bit integer encoded in decimal.
	HeaderTracingParent = "Parent"
	// The Sampled flag, an ASCII "1" (HeaderTracingSampledTrue) if true,
	// otherwise false.
	// If not present, defaults to false.
	HeaderTracingSampled = "Sampled"
	// Trace flags, a 64-bit integer encoded in decimal.
	// If not present, defaults to null.
	HeaderTracingFlags = "Flags"
)

// HeaderTracingSampledTrue is the header value to indicate that this trace
// should be sampled.
const HeaderTracingSampledTrue = "1"

// Deadline propagation related headers.
const (
	// Number of milliseconds, 64-bit integer encoded in decimal.
	HeaderDeadlineBudget = "Deadline-Budget"
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
