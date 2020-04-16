package thriftbp_test

import (
	"time"

	"github.com/reddit/baseplate.go/edgecontext"
	"github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/thriftbp"
)

// This example demonstrates how to use thriftbp.NewServer.
func ExampleNewServer() {
	// variables should be initialized properly in production
	var (
		ecImpl  *edgecontext.Impl
		handler baseplate.BaseplateService
		logger  log.Wrapper
	)

	processor := baseplate.NewBaseplateServiceProcessor(handler)
	server, err := thriftbp.NewServer(
		thriftbp.ServerConfig{
			Addr:    "localhost:8080",
			Timeout: time.Second,
			Logger:  logger,
		},
		processor,
		thriftbp.InjectServerSpan,
		thriftbp.InjectEdgeContext(ecImpl),
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Fatal(server.Serve())
}
