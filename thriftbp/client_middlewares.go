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
// 1. MonitorClient
// 2. ForwardEdgeRequestContext
func BaseplateDefaultClientMiddlewares() []ClientMiddleware {
	return []ClientMiddleware{
		MonitorClient,
		ForwardEdgeRequestContext,
	}
}

// ClientMiddleware can be passed to WrapClient in order to wrap thrift.TClient
// calls with custom middleware.
type ClientMiddleware func(thrift.TClient) thrift.TClient

// WrappedTClient is a convenience struct that implements the thrift.TClient
// interface by calling and returning the inner Wrapped function.
//
// This is provided to aid in developing ClientMiddleware.
type WrappedTClient struct {
	Wrapped func(ctx context.Context, method string, args, result thrift.TStruct) (err error)
}

// Call fulfills the thrift.TClient interface by calling and returning c.Wrapped.
func (c WrappedTClient) Call(ctx context.Context, method string, args, result thrift.TStruct) (err error) {
	return c.Wrapped(ctx, method, args, result)
}

var (
	_ thrift.TClient = WrappedTClient{}
	_ thrift.TClient = (*WrappedTClient)(nil)
)

// WrapClient wraps the given thrift.TClient in the given middlewares.
//
// Middlewares are called in the order they are declared (the first Miiddleware
// passed in is the first/outermost one called).
//
// A typical service should not need to call WrapClient directly, instead you
// should be creating ClientPools using NewBaseplateClientPool which will call
// WrapClient using the BaseplateDefaultMiddlewares() along with any additional
// middleware passed in.
func WrapClient(client thrift.TClient, middlewares ...ClientMiddleware) thrift.TClient {
	for i := len(middlewares) - 1; i >= 0; i-- {
		client = middlewares[i](client)
	}
	return client
}

// MonitorClient is a ClientMiddleware that wraps the inner thrift.TClient.Call
// in a thrift client span.
//
// If you are using a thrift ClientPool created by NewBaseplateClientPool,
// this will be included automatically and should not be passed in as a
// ClientMiddleware to NewBaseplateClientPool.
func MonitorClient(next thrift.TClient) thrift.TClient {
	return WrappedTClient{
		Wrapped: func(ctx context.Context, method string, args, result thrift.TStruct) (err error) {
			var s opentracing.Span
			s, ctx = opentracing.StartSpanFromContext(
				ctx,
				method,
				tracing.SpanTypeOption{Type: tracing.SpanTypeClient},
			)
			span := tracing.AsSpan(s)
			ctx = CreateThriftContextFromSpan(ctx, span)
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
	return WrappedTClient{
		Wrapped: func(ctx context.Context, method string, args, result thrift.TStruct) (err error) {
			if ec, ok := edgecontext.GetEdgeContext(ctx); ok {
				ctx = AttachEdgeRequestContext(ctx, ec)
			}
			return next.Call(ctx, method, args, result)
		},
	}
}

var (
	_ ClientMiddleware = ForwardEdgeRequestContext
	_ ClientMiddleware = MonitorClient
)
