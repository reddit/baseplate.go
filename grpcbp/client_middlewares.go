package grpcbp

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"

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

// PrometheusUnaryClientInterceptor is a client-side interceptor that provides Prometheus
// monitoring for Unary RPCs.
//
// It emits the following metrics:
//
// * grpc_client_active_requests gauge with labels:
//
//   - grpc_service: the fully qualified name of the gRPC service, serviceSlug arg
//   - grpc_method: the name of the method called on the gRPC service
//   - grpc_slug: an arbitray short string representing the backend the client is connecting to
//
// * grpc_client_latency_seconds histogram and grpc_client_requests_total
//   counter with labels:
//
//   - grpc_service, grpc_method, grpc_slug
//   - grpc_success: "true" if status is OK, "false" otherwise
//   - grpc_type: type of request, i.e unary
//   - grpc_code: the human-readable status code, e.g. OK, Internal, etc
func PrometheusUnaryClientInterceptor(serviceSlug, serverSlug string) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req interface{},
		reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) (err error) {
		start := time.Now()
		m := methodSlug(method)
		activeRequestLabels := prometheus.Labels{
			serviceLabel:           serviceSlug,
			methodLabel:            m,
			remoteServiceSlugLabel: serverSlug,
		}
		clientActiveRequests.With(activeRequestLabels).Inc()

		defer func() {
			success := strconv.FormatBool(err == nil)
			status, _ := status.FromError(err)

			latencyLabels := prometheus.Labels{
				serviceLabel:           serviceSlug,
				methodLabel:            m,
				typeLabel:              unary,
				successLabel:           success,
				remoteServiceSlugLabel: serverSlug,
			}

			clientLatencyDistribution.With(latencyLabels).Observe(time.Since(start).Seconds())

			rpcCountLabels := prometheus.Labels{
				serviceLabel:           serviceSlug,
				methodLabel:            m,
				typeLabel:              unary,
				successLabel:           success,
				remoteServiceSlugLabel: serverSlug,
				codeLabel:              status.Code().String(),
			}
			clientRPCRequestCounter.With(rpcCountLabels).Inc()
			clientActiveRequests.With(activeRequestLabels).Dec()
		}()
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// PrometheusStreamClientInterceptor is a client-side interceptor that provides Prometheus
// monitoring for Streaming RPCs.
//
// This is not implemented yet.
func PrometheusStreamClientInterceptor(serviceSlug, serverSlug string) grpc.StreamClientInterceptor {
	return func(
		ctx context.Context,
		desc *grpc.StreamDesc,
		cc *grpc.ClientConn,
		method string,
		streamer grpc.Streamer,
		opts ...grpc.CallOption,
	) (_ grpc.ClientStream, err error) {
		return nil, errors.New("PrometheusStreamClientInterceptor: not implemented")
	}
}
