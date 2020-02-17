package thriftclient

import (
	"context"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/tracing"
)

// MonitoredClient implements the thrift.TClient interface and can be used to
// wrap thrift calls in a client span.
//
// Calls will only be wrapped in client spans if the given context has a parent
// span (either a server or "active" local/client span) set on it.  If no such
// span is set, then the call will still proceed, it will just not be monitored
// by a client span.
type MonitoredClient struct {
	// Client is the inner thrift.TClient implementation wrapped by the
	// MonitoredClient.
	Client thrift.TClient
}

// NewMonitoredClientFromFactory returns a pointer to a new MonitoredClient that
// a new TStandardClient using the provided transport and protocol factory.
func NewMonitoredClientFromFactory(transport thrift.TTransport, factory thrift.TProtocolFactory) *MonitoredClient {
	return &MonitoredClient{thrift.NewTStandardClient(
		factory.GetProtocol(transport),
		factory.GetProtocol(transport),
	)}
}

// Call wraps the underlying thrift.TClient.Call method with a client span.
func (c MonitoredClient) Call(ctx context.Context, method string, args, result thrift.TStruct) (err error) {
	var span *tracing.Span
	if span = tracing.GetActiveSpan(ctx); span == nil {
		span = tracing.GetServerSpan(ctx)
	}
	var child *tracing.Span
	if span != nil {
		ctx, child = span.ChildAndThriftContext(ctx, method)
	}
	defer func() {
		if child != nil {
			if stopErr := child.Stop(ctx, err); stopErr != nil {
				log.Error("Error trying to stop span: " + stopErr.Error())
			}
		}
	}()
	return c.Client.Call(ctx, method, args, result)
}

// Validate that MonitoredClient implements thrift.TClient
var (
	_ thrift.TClient = MonitoredClient{}
	_ thrift.TClient = (*MonitoredClient)(nil)
)
