package thriftbp

import (
	"context"

	"github.com/apache/thrift/lib/go/thrift"
	opentracing "github.com/opentracing/opentracing-go"

	"github.com/reddit/baseplate.go/edgecontext"
	"github.com/reddit/baseplate.go/tracing"
)

// BaseplateDefaultClientMiddlewares returns the default client middlewares that
// should be used by a baseplate service.
//
// Currently they are (in order):
//
// 1. MonitorClient
//
// 2. ForwardEdgeRequestContext
func BaseplateDefaultClientMiddlewares() []thrift.ClientMiddleware {
	return []thrift.ClientMiddleware{
		MonitorClient,
		ForwardEdgeRequestContext,
	}
}

// MonitorClient is a ClientMiddleware that wraps the inner thrift.TClient.Call
// in a thrift client span.
//
// If you are using a thrift ClientPool created by NewBaseplateClientPool,
// this will be included automatically and should not be passed in as a
// ClientMiddleware to NewBaseplateClientPool.
func MonitorClient(next thrift.TClient) thrift.TClient {
	return thrift.WrappedTClient{
		Wrapped: func(ctx context.Context, method string, args, result thrift.TStruct) (err error) {
			span, ctx := opentracing.StartSpanFromContext(
				ctx,
				method,
				tracing.SpanTypeOption{Type: tracing.SpanTypeClient},
			)
			ctx = CreateThriftContextFromSpan(ctx, tracing.AsSpan(span))
			defer func() {
				span.FinishWithOptions(tracing.FinishOptions{
					Ctx: ctx,
					Err: err,
				}.Convert())
			}()

			return next.Call(ctx, method, args, result)
		},
	}
}

// ForwardEdgeRequestContext forwards the EdgeRequestContext set on the context
// object to the Thrift service being called if one is set.
//
// If you are using a thrift ClientPool created by NewBaseplateClientPool,
// this will be included automatically and should not be passed in as a
// ClientMiddleware to NewBaseplateClientPool.
func ForwardEdgeRequestContext(next thrift.TClient) thrift.TClient {
	return thrift.WrappedTClient{
		Wrapped: func(ctx context.Context, method string, args, result thrift.TStruct) (err error) {
			if ec, ok := edgecontext.GetEdgeContext(ctx); ok {
				ctx = AttachEdgeRequestContext(ctx, ec)
			}
			return next.Call(ctx, method, args, result)
		},
	}
}

var (
	_ thrift.ClientMiddleware = ForwardEdgeRequestContext
	_ thrift.ClientMiddleware = MonitorClient
)
