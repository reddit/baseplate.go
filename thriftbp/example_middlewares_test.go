package thriftbp_test

import (
	"github.com/apache/thrift/lib/go/thrift"

	"github.com/reddit/baseplate.go/edgecontext"
	"github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/thriftbp"
	"github.com/reddit/baseplate.go/tracing"
)

// This example demonstrates how to use thriftbp.Middleware with thriftbp.Wraps.
func ExampleWrap() {
	// variables should be initialized properly in production
	var (
		ecImpl    *edgecontext.Impl
		handler   baseplate.BaseplateService
		logger    log.Wrapper
		transport thrift.TServerTransport
	)
	processor := baseplate.NewBaseplateServiceProcessor(handler)
	server := thrift.NewTSimpleServer4(
		// Use thriftbp.Wrap to wrap the base TProcessor in the middleware
		thriftbp.Wrap(
			processor,
			logger,
			edgecontext.InjectThriftEdgeContext(ecImpl, logger),
			tracing.InjectThriftServerSpan,
		),
		transport,
		thrift.NewTHeaderTransportFactory(nil),
		thrift.NewTHeaderProtocolFactory(),
	)
	server.SetForwardHeaders(thriftbp.HeadersToForward)
	server.SetLogger(thrift.Logger(logger))
	log.Fatal(server.Serve())
}
