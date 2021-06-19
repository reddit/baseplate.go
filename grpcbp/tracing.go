package grpcbp

import (
	"context"
	"strconv"

	"github.com/reddit/baseplate.go/tracing"
	"github.com/reddit/baseplate.go/transport"
	"google.golang.org/grpc/metadata"
)

func CreateGRPCContextFromSpan(ctx context.Context, span *tracing.Span) context.Context {
	ctx = metadata.AppendToOutgoingContext(ctx,
		transport.HeaderTracingTrace, span.TraceID(),
		transport.HeaderTracingSpan, span.ID(),
		transport.HeaderTracingFlags, strconv.FormatInt(span.Flags(), 10),
	)
	if span.ParentID() != "" {
		ctx = metadata.AppendToOutgoingContext(ctx,
			transport.HeaderTracingParent, span.ParentID(),
		)
	} else {
		md, _ := metadata.FromIncomingContext(ctx)
		md.Set(transport.HeaderTracingParent, "")
		ctx = metadata.NewOutgoingContext(ctx, md)
	}
	if span.Sampled() {
		ctx = metadata.AppendToOutgoingContext(
			ctx,
			transport.HeaderTracingSampled, transport.HeaderTracingSampledTrue,
		)
	} else {
		md, _ := metadata.FromIncomingContext(ctx)
		md.Set(transport.HeaderTracingSampled, "")
		ctx = metadata.NewOutgoingContext(ctx, md)
	}
	return ctx

}
