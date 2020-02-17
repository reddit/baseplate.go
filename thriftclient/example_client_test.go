package thriftclient_test

import (
	"context"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/thriftclient"
	"github.com/reddit/baseplate.go/tracing"
)

func ExampleMonitoredClient() {
	// variables should be properly initialized in production code
	var (
		transport thrift.TTransport
		factory   thrift.TProtocolFactory
		tracer    *tracing.Tracer
	)
	// Create an actual service client
	client := baseplate.NewBaseplateServiceClient(
		// Use MonitoredClient to wrap a standard thrift client
		thriftclient.NewMonitoredClientFromFactory(transport, factory),
	)
	// Create a context with a server span
	ctx, _ := tracing.CreateServerSpanForContext(context.Background(), tracer, "test")
	// Calls should be automatically wrapped using client spans
	healthy, err := client.IsHealthy(ctx)
	log.Debug("%v, %s", healthy, err)
}
