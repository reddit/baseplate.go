package thriftclient

import (
	"context"

	"github.com/apache/thrift/lib/go/thrift"
	opentracing "github.com/opentracing/opentracing-go"

	"github.com/reddit/baseplate.go/edgecontext"
	"github.com/reddit/baseplate.go/thriftbp"
	"github.com/reddit/baseplate.go/tracing"
)

// BaseplateDefaultMiddlewares returns the default middlewares should be used by
// a baseplate service.
//
// Currently it's (in order):
// - MonitorClient
// - ForwardEdgeRequestContext
func BaseplateDefaultMiddlewares() []Middleware {
	return []Middleware{
		MonitorClient,
		ForwardEdgeRequestContext,
	}
}

// Middleware can be passed to Wrap in order to wrap thrift.TClient calls with
// custom middleware.
type Middleware func(thrift.TClient) thrift.TClient

// WrappedTClient is a convenience struct that implements the thrift.TClient
// interface by calling and returning the inner Wrapped function.
//
// This is provided to aid in developing Middleware.
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

// Wrap wraps the given thrift.TClient in the given middlewares.
//
// Middlewares are called in the order they are declared (the first Miiddleware
// passed in is the first/outermost one called).
//
// A typical service should not need to call Wrap directly, instead you should
// be creating ClientPools using NewBaseplateClientPool which will call Wrap
// using the BaseplateDefaultMiddlewares() along with any additional middleware
// passed in.
func Wrap(client thrift.TClient, middlewares ...Middleware) thrift.TClient {
	for i := len(middlewares) - 1; i >= 0; i-- {
		client = middlewares[i](client)
	}
	return client
}

// MonitorClient is a Middleware that wraps the inner thrift.TClient.Call in
// a thrift client span.
//
// If you are using a thrift ClientPool created by NewBaseplateClientPool,
// this will be included automatically and should not be passed in as a
// Middleware to NewBaseplateClientPool.
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
			ctx = thriftbp.CreateThriftContextFromSpan(ctx, span)
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
// Middleware to NewBaseplateClientPool.
func ForwardEdgeRequestContext(next thrift.TClient) thrift.TClient {
	return WrappedTClient{
		Wrapped: func(ctx context.Context, method string, args, result thrift.TStruct) (err error) {
			if ec, ok := edgecontext.GetEdgeContext(ctx); ok {
				ctx = thriftbp.AttachEdgeRequestContext(ctx, ec)
			}
			return next.Call(ctx, method, args, result)
		},
	}
}

var (
	_ Middleware = ForwardEdgeRequestContext
	_ Middleware = MonitorClient
)
