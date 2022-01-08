package grpcbp

import (
	"context"
	"errors"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/reddit/baseplate.go/ecinterface"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/tracing"
	"github.com/reddit/baseplate.go/transport"
)

// InjectServerSpanInterceptorUnary is a server middleware that injects a server
// span into the `next` context.
//
// If "User-Agent" (transport.HeaderUserAgent) header is set, the created
// server span will also have "peer.service" (tracing.TagKeyPeerService) tag
// set to its value.
func InjectServerSpanInterceptorUnary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (_ interface{}, err error) {
		m := methodSlug(info.FullMethod)
		ctx, span := StartSpanFromGRPCContext(ctx, m)

		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if value, ok := GetHeader(md, transport.HeaderTracingTrace); ok {
				span.SetTag(tracing.TagKeyPeerService, value)
			}
		}

		defer func() {
			span.FinishWithOptions(tracing.FinishOptions{
				Ctx: ctx,
				Err: err,
			}.Convert())
		}()
		return handler(ctx, req)
	}
}

// InjectServerSpanInterceptorStreaming is a server middleware that injects a
// server span into the `next` context.
//
// If "User-Agent" (transport.HeaderUserAgent) header is set, the created
// server span will also have "peer.service" (tracing.TagKeyPeerService) tag
// set to its value.
//
// This is not implemented yet.
func InjectServerSpanInterceptorStreaming() grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		return errors.New("InjectServerSpanInterceptorStreaming: not implemented")
	}
}

// InjectEdgeContextInterceptorUnary is a server middleware that injects an
// edge request context created from the gRPC headers set on the context.
func InjectEdgeContextInterceptorUnary(impl ecinterface.Interface) grpc.UnaryServerInterceptor {
	if impl == nil {
		impl = ecinterface.Get()
	}
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (_ interface{}, err error) {
		ctx = InitializeEdgeContext(ctx, impl)
		return handler(ctx, req)
	}
}

// InjectEdgeContextInterceptorStreaming is a server middleware that injects an
// edge request context created from the gRPC headers set on the context.
//
// This is not implemented yet.
func InjectEdgeContextInterceptorStreaming(impl ecinterface.Interface) grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		return errors.New("InjectEdgeContextInterceptorStreaming: not implemented")
	}
}

// InitializeEdgeContext sets an edge request context created from the gRPC
// headers set on the context onto the context and configures gRPC to forward
// the edge requent context header on any gRPC calls made by the server.
func InitializeEdgeContext(ctx context.Context, impl ecinterface.Interface) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	value, ok := GetHeader(md, transport.HeaderEdgeRequest)
	if !ok {
		return ctx
	}

	ctx, err := impl.HeaderToContext(ctx, value)
	if err != nil {
		log.C(ctx).Errorw(
			"Error while parsing EdgeRequestContext",
			"err", err,
		)
	}
	return ctx
}

// StartSpanFromGRPCContext creates a server span from a gRPC context object.
//
// This span would usually be used as the span of the whole gRPC endpoint
// handler, and the parent of the child-spans.
//
// Caller should pass in the context object they got from gRPC library, which
// would have all the required headers already injected.
//
// Please note that "Sampled" header is default to false according to baseplate
// specification, so if the context object doesn't have headers injected
// correctly, this span (and all its child-spans) will never be sampled, unless
// debug flag was set explicitly later.
//
// If any of the tracing related gRPC header is present but malformed, it will
// be ignored. The error will also be logged if InitGlobalTracer was last
// called with a non-nil logger. Absent tracing related headers are always
// silently ignored.
func StartSpanFromGRPCContext(ctx context.Context, name string) (context.Context, *tracing.Span) {
	var (
		headers tracing.Headers
		sampled bool
	)

	md, _ := metadata.FromIncomingContext(ctx)
	if value, ok := GetHeader(md, transport.HeaderTracingTrace); ok {
		headers.TraceID = value
	}

	if value, ok := GetHeader(md, transport.HeaderTracingSpan); ok {
		headers.SpanID = value
	}

	if value, ok := GetHeader(md, transport.HeaderTracingFlags); ok {
		headers.Flags = value
	}

	if value, ok := GetHeader(md, transport.HeaderTracingSampled); ok {
		sampled = value == transport.HeaderTracingSampled
		headers.Sampled = &sampled
	}

	return tracing.StartSpanFromHeaders(ctx, name, headers)
}

// InjectPrometheusUnaryServerInterceptor is a server middleware that tracks
// Prometheus metrics.
//
// It emits the following metrics:
//
// * grpc_server_active_requests gauge with labels:
//
//   - grpc_service: the fully qualified name of the gRPC service
//   - grpc_method: the name of the method called on the gRPC service
//
// * grpc_server_latency_seconds histogram and grpc_server_requests_total
//   counter with labels:
//
//   - grpc_service and grpc_method
//   - grpc_success: "true" if status is OK, "false" otherwise
//   - grpc_type: type of request, i.e unary
//   - grpc_code: the human-readable status code, e.g. OK, Internal, etc
func InjectPrometheusUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (_ interface{}, err error) {
		start := time.Now()

		serviceName, method := serviceAndMethodSlug(info.FullMethod)

		activeRequestLabels := prometheus.Labels{
			serviceLabel: serviceName,
			methodLabel:  method,
		}
		serverActiveRequests.With(activeRequestLabels).Inc()

		defer func() {
			success := strconv.FormatBool(err == nil)
			status, _ := status.FromError(err)

			latencyLabels := prometheus.Labels{
				serviceLabel: serviceName,
				methodLabel:  method,
				typeLabel:    unary,
				successLabel: success,
			}
			serverLatencyDistribution.With(latencyLabels).Observe(time.Since(start).Seconds())

			rpcCountLabels := prometheus.Labels{
				serviceLabel: serviceName,
				methodLabel:  method,
				typeLabel:    unary,
				successLabel: success,
				codeLabel:    status.Code().String(),
			}
			serverRPCRequestCounter.With(rpcCountLabels).Inc()
			serverActiveRequests.With(activeRequestLabels).Dec()
		}()

		return handler(ctx, req)
	}
}

// InjectPrometheusStreamServerInterceptor is a server middleware that tracks
// Prometheus metrics.
//
// This is not implemented yet.
func InjectPrometheusStreamServerInterceptor(serverSlug string) grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		return errors.New("InjectPrometheusStreamServerInterceptor: not implemented")
	}
}
