package grpcbp

import (
	"context"

	"github.com/reddit/baseplate.go/ecinterface"
	"github.com/reddit/baseplate.go/transport"

	"google.golang.org/grpc/metadata"
)

// AttachEdgeRequestContext returns a context that has the header of the edge
// context attached to ctx object set to forward using the "Edge-Request"
// header on any gRPC calls made with that context object.
func AttachEdgeRequestContext(ctx context.Context, ecImpl ecinterface.Interface) context.Context {
	if ecImpl == nil {
		ecImpl = ecinterface.Get()
	}
	header, ok := ecImpl.ContextToHeader(ctx)
	if !ok {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return ctx
		}
		md.Set(transport.HeaderEdgeRequest, "")
		return metadata.NewOutgoingContext(ctx, md)
	}
	return metadata.AppendToOutgoingContext(
		ctx,
		transport.HeaderEdgeRequest, header,
	)
}
