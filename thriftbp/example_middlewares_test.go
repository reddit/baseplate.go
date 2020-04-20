package thriftbp_test

import (
	"github.com/apache/thrift/lib/go/thrift"

	"github.com/reddit/baseplate.go/edgecontext"
	"github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/thriftbp"
)

// This example demonstrates how to use thriftbp.ProcessorMiddleware with
// thriftbp.WrapProcessor.
func ExampleWrapProcessor() {
	// variables should be initialized properly in production
	var (
		ecImpl    *edgecontext.Impl
		handler   baseplate.BaseplateService
		logger    log.Wrapper
		transport thrift.TServerTransport
	)
	processor := baseplate.NewBaseplateServiceProcessor(handler)
	server := thrift.NewTSimpleServer4(
		// Use thriftbp.WrapProcessor to wrap the base TProcessor in the middleware.
		thriftbp.WrapProcessor(
			processor,
			thriftbp.InjectServerSpan,
			thriftbp.InjectEdgeContext(ecImpl),
		),
		transport,
		thrift.NewTHeaderTransportFactory(nil),
		thrift.NewTHeaderProtocolFactory(),
	)
	server.SetForwardHeaders(thriftbp.HeadersToForward)
	server.SetLogger(thrift.Logger(logger))
	log.Fatal(server.Serve())
}
