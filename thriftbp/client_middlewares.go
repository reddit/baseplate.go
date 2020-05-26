package thriftbp

import (
	"context"
	"strconv"
	"time"

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
// 1. ForwardEdgeRequestContext
//
// 2. MonitorClient
//
// 3. SetDeadlineBudget
func BaseplateDefaultClientMiddlewares(service string) []thrift.ClientMiddleware {
	return []thrift.ClientMiddleware{
		ForwardEdgeRequestContext,
		MonitorClient(service),
		SetDeadlineBudget,
	}
}

// MonitorClient is a ClientMiddleware that wraps the inner thrift.TClient.Call
// in a thrift client span.
//
// If you are using a thrift ClientPool created by NewBaseplateClientPool,
// this will be included automatically and should not be passed in as a
// ClientMiddleware to NewBaseplateClientPool.
func MonitorClient(service string) thrift.ClientMiddleware {
	prefix := service + "."
	return func(next thrift.TClient) thrift.TClient {
		return thrift.WrappedTClient{
			Wrapped: func(ctx context.Context, method string, args, result thrift.TStruct) (err error) {
				span, ctx := opentracing.StartSpanFromContext(
					ctx,
					prefix+method,
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

// SetDeadlineBudget is the client middleware implementing Phase 1 of Baseplate
// deadline propogation.
func SetDeadlineBudget(next thrift.TClient) thrift.TClient {
	return thrift.WrappedTClient{
		Wrapped: func(ctx context.Context, method string, args, result thrift.TStruct) error {
			if ctx.Err() != nil {
				// Deadline already passed, no need to even try
				return ctx.Err()
			}

			if deadline, ok := ctx.Deadline(); ok {
				// Round up to the next millisecond.
				// In the scenario that the caller set an 10ms timeout and send the
				// request, by the time we get into this middleware function it's
				// definitely gonna be less than 10ms.
				// If we use round down then we are only gonna send 9 over the wire.
				timeout := deadline.Sub(time.Now()) + time.Millisecond - 1
				ms := timeout.Milliseconds()
				if ms < 1 {
					// Make sure we give it at least 1ms.
					ms = 1
				}
				value := strconv.FormatInt(ms, 10)
				ctx = thrift.SetHeader(ctx, HeaderDeadlineBudget, value)
			}

			return next.Call(ctx, method, args, result)
		},
	}
}

var (
	_ thrift.ClientMiddleware = ForwardEdgeRequestContext
	_ thrift.ClientMiddleware = SetDeadlineBudget
)
