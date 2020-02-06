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
//
// Note, this function depends on the edge context headers already being set on
// the context object.  This can be done by adding httpbp.PopulateRequestContext
// as a ServerBefore option when setting up the request handler for an endpoint.
func InjectHTTPServerSpan(name string) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request interface{}) (response interface{}, err error) {
			ctx, span := StartSpanFromHTTPContext(ctx, name)
			defer span.Stop(ctx, err)

			response, err = next(ctx, request)
			return
		}
	}
}

// InjectHTTPServerSpanWithTracer is the same as InjectHTTPServerSpan except it
// uses StartSpanFromHTTPContextWithTracer to initialize the server span rather
// than StartSpanFromHTTPContext.
func InjectHTTPServerSpanWithTracer(tracer *Tracer, name string) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request interface{}) (response interface{}, err error) {
			ctx, span := StartSpanFromHTTPContextWithTracer(ctx, name, tracer)
			defer span.Stop(ctx, err)

			response, err = next(ctx, request)
			return
		}
	}
}
