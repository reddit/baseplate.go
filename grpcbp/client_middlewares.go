package grpcbp

import (
	"context"
	"errors"

	"github.com/opentracing/opentracing-go"
	"google.golang.org/grpc"

	"github.com/reddit/baseplate.go/ecinterface"
	"github.com/reddit/baseplate.go/tracing"
)

// MonitorInterceptorArgs are the arguments to be passed into the
// MonitorInterceptorUnary function.
type MonitorInterceptorArgs struct {
	ServiceSlug string
}

// MonitorInterceptorUnary is a client middleware that provides tracing and
// metrics by starting or continuing a span.
func MonitorInterceptorUnary(args MonitorInterceptorArgs) grpc.UnaryClientInterceptor {
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
		m := methodSlug(method)
		span, ctx := opentracing.StartSpanFromContext(
			ctx,
			prefix+m,
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

// MonitorInterceptorStreaming is a client middleware that provides tracing and
// metrics by starting or continuing a span.
//
// This is not implemented yet.
func MonitorInterceptorStreaming(args MonitorInterceptorArgs) grpc.StreamClientInterceptor {
	return func(
		ctx context.Context,
		desc *grpc.StreamDesc,
		cc *grpc.ClientConn,
		method string,
		streamer grpc.Streamer,
		opts ...grpc.CallOption,
	) (grpc.ClientStream, error) {
		return nil, errors.New("grpc.MonitorInterceptorStreaming: not implemented")
	}
}

// ForwardEdgeContextUnary is a client middleware that forwards the
// EdgeRequestContext set on the context object to the gRPC service being
// called if one is set.
func ForwardEdgeContextUnary(ecImpl ecinterface.Interface) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req interface{},
		reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) (err error) {
		ctx = AttachEdgeRequestContext(ctx, ecImpl)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// ForwardEdgeContextStreaming is a client middleware that forwards the
// EdgeRequestContext set on the context object to the gRPC service being
// called if one is set.
//
// This is not implemented yet.
func ForwardEdgeContextStreaming(ecImpl ecinterface.Interface) grpc.StreamClientInterceptor {
	return func(
		ctx context.Context,
		desc *grpc.StreamDesc,
		cc *grpc.ClientConn,
		method string,
		streamer grpc.Streamer,
		opts ...grpc.CallOption,
	) (grpc.ClientStream, error) {
		return nil, errors.New("grpc.ForwardEdgeContextStreaming: not implemented")
	}
}
