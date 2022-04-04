package thriftbp

import (
	"context"
	"strings"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/reddit/baseplate.go/ecinterface"
	"github.com/reddit/baseplate.go/transport"
)

// HeadersToForward are the headers that should always be forwarded to upstream
// thrift servers, to be used in thrift.TSimpleServer.SetForwardHeaders.
var HeadersToForward = []string{
	transport.HeaderEdgeRequest,
	transport.HeaderTracingTrace,
	transport.HeaderTracingSpan,
	transport.HeaderTracingParent,
	transport.HeaderTracingSampled,
	transport.HeaderTracingFlags,
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
		return thrift.UnsetHeader(ctx, transport.HeaderEdgeRequest)
	}
	return AddClientHeader(ctx, transport.HeaderEdgeRequest, header)
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

// header gets the value of a thrift header by key
//
// If the value is not present we fall back to all lowercase check to workaround a bug in envoy
func header(ctx context.Context, key string) (v string, ok bool) {
	v, ok = thrift.GetHeader(ctx, key)
	if !ok {
		// We fall back to checking the key that an envoy proxy will send, which is a lower-case
		// version of the thrift key.
		//
		//
		// For details on this workaround, see:
		// 	* https://github.com/reddit/baseplate.py/commit/daf1c5e32dcc019470d75f3940006bb2f7b9855e
		// 	* https://github.com/envoyproxy/envoy/issues/20595
		// 	* https://github.com/reddit/baseplate.go/pull/500
		v, ok = thrift.GetHeader(ctx, strings.ToLower(key))
	}

	return
}
