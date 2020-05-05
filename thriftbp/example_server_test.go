package thriftbp_test

import (
	"context"

	baseplate "github.com/reddit/baseplate.go"
	bpgen "github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/thriftbp"
)

// This example demonstrates how to use thriftbp.NewServer.
func ExampleNewBaseplateServer() {
	// variables should be initialized properly in production
	var handler bpgen.BaseplateService
	ctx := context.Background()
	bp, err := baseplate.New(ctx, "example.yaml", nil)
	if err != nil {
		panic(err)
	}
	defer bp.Close()

	processor := bpgen.NewBaseplateServiceProcessor(handler)
	server, err := thriftbp.NewBaseplateServer(bp, processor)
	if err != nil {
		log.Fatal(err)
	}

	log.Info(baseplate.Serve(ctx, server))
}
