package edgecontext

import (
	"context"
	"errors"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/thriftbp"
)

// InitializeEdgeContext sets an edge request context created using the given
// ContextFactory onto the context.
func InitializeEdgeContext(ctx context.Context, impl *Impl, logger log.Wrapper, factory ContextFactory) context.Context {
	if ec, err := factory(ctx, impl); err == nil && ec != nil {
		ctx = SetEdgeContext(ctx, ec)
	} else if !errors.Is(err, ErrNoHeader) && logger != nil {
		logger("Error while trying to initialize edge context: " + err.Error())
	}
	return ctx
}

// InitializeThriftEdgeContext sets an edge request context created from the
// Thrift headers set on the context onto the context.
func InitializeThriftEdgeContext(ctx context.Context, impl *Impl, logger log.Wrapper) context.Context {
	return InitializeEdgeContext(ctx, impl, logger, FromThriftContext)
}

// InjectThriftEdgeContext returns a thriftbp.Middleware that injects an edge
// request context created from the Thrift headers set on the context into the
// `next` thrift.TProcessorFunction.
//
// Note, this depends on the edge context headers already being set on the
// context object.  These should be automatically injected by your
// thrift.TSimpleServer.
func InjectThriftEdgeContext(impl *Impl, logger log.Wrapper) thriftbp.Middleware {
	return func(name string, next thrift.TProcessorFunction) thrift.TProcessorFunction {
		return thriftbp.WrappedTProcessorFunc{
			Wrapped: func(ctx context.Context, seqId int32, in, out thrift.TProtocol) (bool, thrift.TException) {
				ctx = InitializeThriftEdgeContext(ctx, impl, logger)
				return next.Process(ctx, seqId, in, out)
			},
		}
	}
}
