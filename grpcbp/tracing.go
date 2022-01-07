package grpcbp

import (
	"context"
	"strconv"
	"strings"

	"google.golang.org/grpc/metadata"

	"github.com/reddit/baseplate.go/tracing"
	"github.com/reddit/baseplate.go/transport"
)

// CreateGRPCContextFromSpan injects span info into a context object that can
// be used in gRPC client code.
func CreateGRPCContextFromSpan(ctx context.Context, span *tracing.Span) context.Context {
	kvs := []string{
		transport.HeaderTracingTrace, span.TraceID(),
		transport.HeaderTracingSpan, span.ID(),
		transport.HeaderTracingFlags, strconv.FormatInt(span.Flags(), 10),
	}

	if span.ParentID() != "" {
		kvs = append(kvs, transport.HeaderTracingParent, span.ParentID())
	} else {
		md, _ := metadata.FromIncomingContext(ctx)
		md.Delete(transport.HeaderTracingParent)
		ctx = metadata.NewOutgoingContext(ctx, md)
	}

	if span.Sampled() {
		kvs = append(kvs, transport.HeaderTracingSampled, transport.HeaderTracingSampledTrue)
	} else {
		md, _ := metadata.FromIncomingContext(ctx)
		md.Delete(transport.HeaderTracingSampled)
		ctx = metadata.NewOutgoingContext(ctx, md)
	}

	return metadata.AppendToOutgoingContext(ctx, kvs...)

}

func methodSlug(method string) string {
	split := strings.Split(method, "/")
	return split[len(split)-1]
}

// serviceAndMethodSlug splits the UnaryServerInfo.FullMethod and returns
// the package.service part separate from the method part.
// ref: https://pkg.go.dev/google.golang.org/grpc#UnaryServerInfo
func serviceAndMethodSlug(fullMethod string) (string, string) {
	split := strings.Split(fullMethod, "/")
	method := split[len(split)-1]
	service := strings.Join(split[:len(split)-1], "")
	return service, method
}
