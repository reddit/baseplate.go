package tracing

import (
	"context"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/reddit/baseplate.go/thriftbp"
)

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
