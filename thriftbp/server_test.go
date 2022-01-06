package thriftbp_test

import (
	"context"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/reddit/baseplate.go"
	"github.com/reddit/baseplate.go/thriftbp"
)

func ExampleNewServerFromOptions() {

	var processor thrift.TProcessor
	var myMiddleware1 thrift.ProcessorMiddleware
	var myMiddleware2 thrift.ProcessorMiddleware
	var socket *thrift.TServerSocket
	var logger thrift.Logger

	_, bp, _ := baseplate.New(context.Background(), baseplate.NewArgs{})

	server, _ := thriftbp.NewServerFromOptions(
		thriftbp.WithProcessor(processor),
		thriftbp.WithErrorSpanSuppressor(func(err error) bool {
			return false
		}),
		thriftbp.WithPayloadSizeMetricsSampleRate(0.5),
		thriftbp.WithDefaultMiddleware(bp),
		thriftbp.WithMiddleware(myMiddleware1, myMiddleware2),
		thriftbp.WithLogger(logger),
		thriftbp.WithSocket(socket),
	)

	_ = thriftbp.ApplyBaseplate(bp, server)
}

func ExampleNewBaseplateServerFromOptions() {

	var processor thrift.TProcessor
	var myMiddleware1 thrift.ProcessorMiddleware
	var myMiddleware2 thrift.ProcessorMiddleware
	var socket *thrift.TServerSocket
	var logger thrift.Logger

	_, bp, _ := baseplate.New(context.Background(), baseplate.NewArgs{})

	serverOptions := []thriftbp.ServerOption{
		thriftbp.WithProcessor(processor),
		thriftbp.WithErrorSpanSuppressor(func(err error) bool {
			return false
		}),
		thriftbp.WithPayloadSizeMetricsSampleRate(0.5),
	}
	serverOptions = append(serverOptions, thriftbp.DefaultServerOptions(bp)...)
	serverOptions = append(serverOptions,
		thriftbp.WithMiddleware(myMiddleware1, myMiddleware2),
		thriftbp.WithLogger(logger),
		thriftbp.WithSocket(socket),
	)

	_, _ = thriftbp.NewBaseplateServerFromOptions(bp, serverOptions...)

}
