package thriftclient

import (
	"context"

	"github.com/apache/thrift/lib/go/thrift"
	opentracing "github.com/opentracing/opentracing-go"

	"github.com/reddit/baseplate.go/metricsbp"
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
	var s opentracing.Span
	s, ctx = opentracing.StartSpanFromContext(
		ctx,
		method,
		tracing.SpanTypeOption{Type: tracing.SpanTypeClient},
	)
	span := tracing.AsSpan(s)
	ctx = tracing.CreateThriftContextFromSpan(ctx, span)
	defer func() {
		span.FinishWithOptions(tracing.FinishOptions{
			Ctx: ctx,
			Err: err,
		}.Convert())
		if err != nil {
			metricsbp.M.Counter(method + ".error").Add(1)
		}
	}()
	return c.Client.Call(ctx, method, args, result)
}

// Validate that MonitoredClient implements thrift.TClient
var (
	_ thrift.TClient = MonitoredClient{}
	_ thrift.TClient = (*MonitoredClient)(nil)
)
