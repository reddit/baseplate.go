package grpcbp

import (
	"context"

	"google.golang.org/grpc/metadata"

	"github.com/reddit/baseplate.go/ecinterface"
	"github.com/reddit/baseplate.go/transport"
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
		md.Delete(transport.HeaderEdgeRequest)
		return metadata.NewOutgoingContext(ctx, md)
	}
	return metadata.AppendToOutgoingContext(
		ctx,
		transport.HeaderEdgeRequest, header,
	)
}

// GetHeader retrieves the header value for a given key. Since metadata.MD
// headers are mapped to a list of strings this function checks if there is at
// least one value present.
func GetHeader(md metadata.MD, key string) (string, bool) {
	if values := md.Get(key); len(values) > 0 && values[0] != "" {
		return values[0], true
	}
	return "", false
}
