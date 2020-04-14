package runtimebp_test

import (
	"context"
	"os"

	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/runtimebp"
	"github.com/reddit/baseplate.go/thriftbp"
)

// This example demonstrates how to handle graceful shutdown with
// HandleShutdown.
func ExampleHandleShutdown() {
	var (
		// TODO: In real code this should be coming from thrift compiled go files.
		processor thriftbp.BaseplateProcessor
	)
	server, err := thriftbp.NewServer(
		thriftbp.ServerConfig{
			Addr: "localhost:8080",
			// TODO: Add other configs
		},
		processor,
	)
	if err != nil {
		log.Fatal(err)
	}
	go runtimebp.HandleShutdown(
		context.Background(),
		func(signal os.Signal) {
			log.Infow(
				"graceful shutdown",
				"signal", signal,
				"stop error", server.Stop(),
			)
		},
	)
	log.Info(server.Serve())
}
