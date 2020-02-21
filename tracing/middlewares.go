package tracing

import (
	"context"

	"github.com/go-kit/kit/endpoint"
)

// InjectHTTPServerSpan returns a go-kit endpoint.Middleware that injects a server
// span into the `next` context.
//
// Starts the server span before calling the `next` endpoint and stops the span
// after the endpoint finishes.
// If the endpoint returns an error, that will be passed to span.Stop. If the
// response implements ErrorResponse, the error returned by Err() will not be
// passed to span.Stop.
//
// Note, this function depends on the edge context headers already being set on
// the context object.  This can be done by adding httpbp.PopulateRequestContext
// as a ServerBefore option when setting up the request handler for an endpoint.
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

// InjectThriftServerSpan returns a go-kit endpoint.Middleware that injects a server
// span into the `next` context.
//
// Starts the server span before calling the `next` endpoint and stops the span
// after the endpoint finishes.
// If the endpoint returns an error, that will be passed to span.Stop.
//
// Note, this depends on the edge context headers already being set on the
// context object.  These should be automatically injected by your
// thrift.TSimpleServer.
func InjectThriftServerSpan(name string) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request interface{}) (response interface{}, err error) {
			ctx, span := StartSpanFromThriftContext(ctx, name)
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
