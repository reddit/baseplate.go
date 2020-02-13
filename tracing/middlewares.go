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
	return InjectHTTPServerSpanWithTracer(name, nil)
}

// InjectHTTPServerSpanWithTracer is the same as InjectHTTPServerSpan except it
// uses StartSpanFromHTTPContextWithTracer to initialize the server span rather
// than StartSpanFromHTTPContext.
func InjectHTTPServerSpanWithTracer(name string, tracer *Tracer) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request interface{}) (response interface{}, err error) {
			ctx, span := StartSpanFromHTTPContextWithTracer(ctx, name, tracer)
			defer func() {
				span.Stop(ctx, err)
			}()

			response, err = next(ctx, request)
			return
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
	return InjectThriftServerSpanWithTracer(name, nil)
}

// InjectThriftServerSpanWithTracer is the same as InjectThriftServerSpan except it
// uses StartSpanFromThriftContextWithTracer to initialize the server span rather
// than StartSpanFromThriftContext.
func InjectThriftServerSpanWithTracer(name string, tracer *Tracer) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request interface{}) (response interface{}, err error) {
			ctx, span := StartSpanFromThriftContextWithTracer(ctx, name, tracer)
			defer func() {
				if err = span.Stop(ctx, err); err != nil && tracer.Logger != nil {
					tracer.Logger("Error while trying to stop span: " + err.Error())
				}
			}()

			response, err = next(ctx, request)
			return
		}
	}
}
