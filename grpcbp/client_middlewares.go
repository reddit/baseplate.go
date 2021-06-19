package grpcbp

import (
	"context"

	"github.com/opentracing/opentracing-go"
	"github.com/reddit/baseplate.go/tracing"
	"google.golang.org/grpc"
)

type MonitorClientArgs struct {
	ServiceSlug string
}

func TraceInterceptor(args MonitorClientArgs) grpc.UnaryClientInterceptor {
	prefix := args.ServiceSlug + "."
	return func(
		ctx context.Context,
		method string,
		req interface{},
		reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) (err error) {
		span, ctx := opentracing.StartSpanFromContext(
			ctx,
			prefix+method,
			tracing.SpanTypeOption{
				Type: tracing.SpanTypeClient,
			},
		)
		ctx = CreateGRPCContextFromSpan(ctx, tracing.AsSpan(span))
		defer func() {
			span.FinishWithOptions(tracing.FinishOptions{
				Ctx: ctx,
				Err: err,
			}.Convert())
		}()
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
