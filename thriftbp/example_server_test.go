package thriftbp_test

import (
	"context"

	baseplate "github.com/reddit/baseplate.go"
	bpgen "github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/thriftbp"
)

// This example demonstrates what a typical main function should look like for a
// Baseplate thrift service.
func ExampleNewBaseplateServer() {
	ctx, bp, err := baseplate.New(context.Background(), "example.yaml", nil)
	if err != nil {
		panic(err)
	}
	defer bp.Close()

	// In real prod code, you should define your thrift endpoints and create this
	// handler instead.
	var handler bpgen.BaseplateServiceV2
	processor := bpgen.NewBaseplateServiceV2Processor(handler)

	server, err := thriftbp.NewBaseplateServer(bp, thriftbp.ServerConfig{
		Processor: processor,
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Info(baseplate.Serve(ctx, server))
}
