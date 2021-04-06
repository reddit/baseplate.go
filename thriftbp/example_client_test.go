package thriftbp_test

import (
	"context"

	"github.com/apache/thrift/lib/go/thrift"
	retry "github.com/avast/retry-go"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/retrybp"
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
	// Use MonitoredClient to wrap a standard thrift client
	//
	// The TClient returned by thrift.WrapClient *could* be shareable across
	// multiple goroutines. For example, the one create here using protocol
	// factory, or the one created via
	// thriftbp.NewBaseplateClientPool/NewCustomClientPool.
	shareableClient := thrift.WrapClient(
		thrift.NewTStandardClient(
			factory.GetProtocol(transport),
			factory.GetProtocol(transport),
		),
		thriftbp.MonitorClient(thriftbp.MonitorClientArgs{
			ServiceSlug: "service",
		}),
	)
	// Create an actual service client
	//
	// The actual client is NOT shareable between multiple goroutines and you
	// should always create one on top of the shareable TClient for each call.
	client := baseplate.NewBaseplateServiceV2Client(shareableClient)
	// Create a context with a server span
	_, ctx := opentracing.StartSpanFromContext(
		context.Background(),
		"test",
		tracing.SpanTypeOption{Type: tracing.SpanTypeServer},
	)
	// Calls should be automatically wrapped using client spans
	healthy, err := client.IsHealthy(
		// The default middleware does not automatically retry requests but does set
		// up the retry middleware so individual requests can be configured to retry
		// using retrybp.WithOptions.
		retrybp.WithOptions(
			ctx,
			// This call will make at most 2 attempts, that is the initial attempt and
			// a single retry.
			retry.Attempts(2),
			// Apply the thriftbp default retry filters as well as NetworkErrorFilter
			// to retry networking errors.
			//
			// NetworkErrorFilter should only be used for requests that are safe to
			// repeat, such as reads or idempotent requests.
			retrybp.Filters(
				thriftbp.WithDefaultRetryFilters(retrybp.NetworkErrorFilter)...,
			),
		),
		&baseplate.IsHealthyRequest{
			Probe: baseplate.IsHealthyProbePtr(baseplate.IsHealthyProbe_READINESS),
		},
	)
	log.Debug("%v, %s", healthy, err)
}
