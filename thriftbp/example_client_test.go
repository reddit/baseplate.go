package thriftbp_test

import (
	"context"

	"github.com/apache/thrift/lib/go/thrift"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/thriftbp"
	"github.com/reddit/baseplate.go/tracing"
)

// This example illustrates what thriftbp.MonitorClient does specifically and
// the details of how thriftbp.WrapClient works,
// a typical service will not write code like this and will instead be creating
// a ClientPool using thriftbp.NewBaseplateClientPool.
func ExampleMonitorClient() {
	// variables should be properly initialized in production code
	var (
		transport thrift.TTransport
		factory   thrift.TProtocolFactory
	)
	// Create an actual service client
	client := baseplate.NewBaseplateServiceClient(
		// Use MonitoredClient to wrap a standard thrift client
		thrift.WrapClient(
			thrift.NewTStandardClient(
				factory.GetProtocol(transport),
				factory.GetProtocol(transport),
			),
			thriftbp.MonitorClient("service"),
		),
	)
	// Create a context with a server span
	_, ctx := opentracing.StartSpanFromContext(
		context.Background(),
		"test",
		tracing.SpanTypeOption{Type: tracing.SpanTypeServer},
	)
	// Calls should be automatically wrapped using client spans
	healthy, err := client.IsHealthy(ctx)
	log.Debug("%v, %s", healthy, err)
}
