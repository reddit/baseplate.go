package thriftbp

import (
	"context"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/reddit/baseplate.go/ecinterface"
	"github.com/reddit/baseplate.go/transport"
)

// Edge request context propagation related headers, as defined in
// https://pages.github.snooguts.net/reddit/baseplate.spec/component-apis/thrift#edge-request-context-propagation
//
// Deprecated: use transport.HeaderEdgeRequest instead
const (
	HeaderEdgeRequest = transport.HeaderEdgeRequest
)

// Tracing related headers, as defined in
// https://pages.github.snooguts.net/reddit/baseplate.spec/component-apis/thrift#tracing
const (
	// Deprecated: use transport.HeaderTracingTrace instead
	HeaderTracingTrace = transport.HeaderTracingTrace
	// Deprecated: use transport.HeaderTracingSpan instead
	HeaderTracingSpan = transport.HeaderTracingSpan
	// Deprecated: use transport.HeaderTracingParent instead
	HeaderTracingParent = transport.HeaderTracingParent
	// Deprecated: use transport.HeaderTracingSampled instead
	HeaderTracingSampled = transport.HeaderTracingSampled
	// Deprecated: use transport.HeaderTracingFlags instead
	HeaderTracingFlags = transport.HeaderTracingFlags
)

// HeaderTracingSampledTrue is the header value to indicate that this trace
// should be sampled.
const (
	// Deprecated: use transport.HeaderTracingSampledTrue instead
	HeaderTracingSampledTrue = transport.HeaderTracingSampledTrue
)

// Deadline propagation related headers.
const (
	// Deprecated: use transport.HeaderDeadlineBudget instead
	HeaderDeadlineBudget = transport.HeaderDeadlineBudget
)

// UserAgent related headers.
const (
	// Deprecated: use transport.HeaderUserAgent instead
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
