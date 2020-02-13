package edgecontext

import (
	"context"
	"errors"

	"github.com/go-kit/kit/endpoint"

	"github.com/reddit/baseplate.go/log"
)

// InitializeEdgeContext sets an edge request context created using the given
// ContextFactory onto the context.
func InitializeEdgeContext(ctx context.Context, logger log.Wrapper, factory ContextFactory) context.Context {
	if ec, err := factory(ctx); err == nil && ec != nil {
		ctx = SetEdgeContext(ctx, ec)
	} else if !errors.Is(err, ErrNoHeader) && logger != nil {
		logger("Error while trying to initialize edge context: " + err.Error())
	}
	return ctx
}

// InitializeHTTPEdgeContext sets an edge request context created from the HTTP
// headers set on the context onto the context.
func InitializeHTTPEdgeContext(ctx context.Context, logger log.Wrapper) context.Context {
	return InitializeEdgeContext(ctx, logger, FromHTTPContext)
}

// InjectHTTPEdgeContext returns a go-kit endpoint.Middleware that injects an
// edge request context created from the HTTP headers set on the context into
// the `next` endpoint.Endpoint.
//
// Note, this depends on the edge context headers already being set on the
// context object.  This can be done by adding httpbp.PopulateRequestContext as
// a ServerBefore option when setting up the request handler for an endpoint.
func InjectHTTPEdgeContext(logger log.Wrapper) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request interface{}) (interface{}, error) {
			return next(InitializeHTTPEdgeContext(ctx, logger), request)
		}
	}
}

// InitializeThriftEdgeContext sets an edge request context created from the
// Thrift headers set on the context onto the context.
func InitializeThriftEdgeContext(ctx context.Context, logger log.Wrapper) context.Context {
	return InitializeEdgeContext(ctx, logger, FromThriftContext)
}

// InjectThriftEdgeContext returns a go-kit endpoint.Middleware that injects an
// edge request context created from the Thrift headers set on the context into
// the `next` endpoint.Endpoint.
//
// Note, this depends on the edge context headers already being set on the
// context object.  These should be automatically injected by your
// thrift.TSimpleServer.
func InjectThriftEdgeContext(logger log.Wrapper) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request interface{}) (interface{}, error) {
			return next(InitializeThriftEdgeContext(ctx, logger), request)
		}
	}
}
