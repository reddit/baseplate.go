package thriftbp

import (
	"context"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/reddit/baseplate.go/internal/faults"
)

type clientFaultMiddleware struct {
	address  string
	injector faults.Injector[thrift.ResponseMeta]
}

// NewClientFaultMiddleware creates and returns a new client-side fault
// injection middleware.
func NewClientFaultMiddleware(clientName, address string) clientFaultMiddleware {
	return clientFaultMiddleware{
		address: address,
		injector: *faults.NewInjector(
			clientName,
			"thriftpb.clientFaultMiddleware",
			thrift.UNKNOWN_TRANSPORT_EXCEPTION,
			thrift.END_OF_FILE,
			faults.WithDefaultAbort(func(code int, message string) (thrift.ResponseMeta, error) {
				return thrift.ResponseMeta{}, thrift.NewTTransportException(code, message)
			}),
		),
	}
}

type thriftHeaders struct{}

// Lookup returns the value of the header, if found.
func (h thriftHeaders) Lookup(ctx context.Context, key string) (string, error) {
	header, ok := thrift.GetHeader(ctx, key)
	if !ok {
		return "", nil
	}
	return header, nil
}
