package grpcbp

import (
	"context"

	"github.com/reddit/baseplate.go/ecinterface"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/tracing"
	"github.com/reddit/baseplate.go/transport"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func InjectServerSpanInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (_ interface{}, err error) {
		ctx, span := StartSpanFromGRPCContext(ctx, info.FullMethod)
		defer func() {
			span.FinishWithOptions(tracing.FinishOptions{
				Ctx: ctx,
				Err: err,
			}.Convert())
		}()
		return handler(ctx, req)
	}
}

func InjectEdgeContextInterceptor(impl ecinterface.Interface) grpc.UnaryServerInterceptor {
	if impl == nil {
		impl = ecinterface.Get()
	}
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (_ interface{}, err error) {
		ctx = InitializeEdgeContext(ctx, impl)
		return handler(ctx, req)
	}
}

func InitializeEdgeContext(ctx context.Context, impl ecinterface.Interface) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	values, ok := md[transport.HeaderEdgeRequest]
	if !ok {
		return ctx
	}

	ctx, err := impl.HeaderToContext(ctx, values[0])
	if err != nil {
		log.C(ctx).Errorw("Error while parsing EdgeRequestContext: " + err.Error())
	}
	return ctx
}

func StartSpanFromGRPCContext(ctx context.Context, name string) (context.Context, *tracing.Span) {
	var headers tracing.Headers
	var sampled bool

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return tracing.StartSpanFromHeaders(ctx, name, headers)
	}

	values := md.Get(transport.HeaderTracingTrace)
	headers.TraceID = values[0]

	values = md.Get(transport.HeaderTracingSpan)
	headers.SpanID = values[0]

	values = md.Get(transport.HeaderTracingFlags)
	headers.Flags = values[0]

	values = md.Get(transport.HeaderTracingSampled)
	sampled = values[0] == transport.HeaderTracingSampledTrue
	headers.Sampled = &sampled

	return tracing.StartSpanFromHeaders(ctx, name, headers)
}
