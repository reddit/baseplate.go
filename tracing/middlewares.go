package tracing

import (
	"context"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/go-kit/kit/endpoint"

	"github.com/reddit/baseplate.go/thriftbp"
)

// InjectHTTPServerSpan returns a go-kit endpoint.Middleware that injects a
// server span into the `next` context.
//
// Starts the server span before calling the `next` endpoint and stops the span
// after the endpoint finishes.
// If the endpoint returns an error, that will be passed to span.Stop. If the
// response implements ErrorResponse, the error returned by Err() will not be
// passed to span.Stop.
//
// Note, this function depends on the tracing headers already being set on the
// context object.
// This can be done by adding httpbp.PopulateRequestContext as a ServerBefore
// option when setting up the request handler for an endpoint.
func InjectHTTPServerSpan(name string) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request interface{}) (response interface{}, err error) {
			ctx, span := StartSpanFromHTTPContext(ctx, name)
			defer func() {
				span.FinishWithOptions(FinishOptions{
					Ctx: ctx,
					Err: err,
				}.Convert())
			}()

			return next(ctx, request)
		}
	}
}

// InjectThriftServerSpan implements thriftbp.Middleware and injects a server
// span into the `next` context.
//
// Starts the server span before calling the `next` TProcessorFunction and stops
// the span after it finishes.
// If the function returns an error, that will be passed to span.Stop.
//
// Note, the span will be created according to tracing related headers already
// being set on the context object.
// These should be automatically injected by your thrift.TSimpleServer.
func InjectThriftServerSpan(name string, next thrift.TProcessorFunction) thrift.TProcessorFunction {
	return thriftbp.WrappedTProcessorFunc{
		Wrapped: func(ctx context.Context, seqId int32, in, out thrift.TProtocol) (success bool, err thrift.TException) {
			ctx, span := StartSpanFromThriftContext(ctx, name)
			defer func() {
				span.FinishWithOptions(FinishOptions{
					Ctx: ctx,
					Err: err,
				}.Convert())
			}()

			return next.Process(ctx, seqId, in, out)
		},
	}
}

var (
	_ thriftbp.Middleware = InjectThriftServerSpan
)
