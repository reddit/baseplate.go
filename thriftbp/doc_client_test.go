package thriftbp_test

import (
	"context"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/thriftbp"
)

// In real code these should be coming from either config file or flags instead.
const (
	remoteAddr     = "host:port"
	connectTimeout = time.Millisecond * 5
	socketTimeout  = time.Millisecond * 15

	initialConnections = 50
	maxConnections     = 100

	clientTTL = time.Minute * 5

	poolGaugeInterval = time.Second * 10
)

// BEGIN THRIFT GENERATED CODE SECTION
//
// In real code this section should be from thrift generated code instead,
// but for this example we just define some placeholders here.

type MyEndpointRequest struct{}

type MyEndpointResponse struct{}

type MyService interface {
	MyEndpoint(ctx context.Context, req *MyEndpointRequest) (*MyEndpointResponse, error)
}

func NewMyServiceClient(_ thrift.TClient) MyService {
	// In real code this certainly won't return nil.
	return nil
}

// END THRIFT GENERATED CODE SECTION

func LoggingMiddleware(next thrift.TClient) thrift.TClient {
	return thrift.WrappedTClient{
		Wrapped: func(ctx context.Context, method string, args, result thrift.TStruct) error {
			log.Infof("pre: %s", method)
			log.Infof("args: %#v", args)
			defer func() {
				log.Infof("after: %s", method)
			}()

			return next.Call(ctx, method, args, result)
		},
	}
}

// This example demonstrates a typical use case of thriftbp pool in
// microservice code with custom middleware.
func Example_clientPool() {
	pool, err := thriftbp.NewBaseplateClientPool(
		thriftbp.ClientPoolConfig{
			ServiceSlug:        "my-service",
			Addr:               remoteAddr,
			InitialConnections: initialConnections,
			MaxConnections:     maxConnections,
			MaxConnectionAge:   clientTTL,
			ConnectTimeout:     connectTimeout,
			SocketTimeout:      socketTimeout,
			ReportPoolStats:    true,
			PoolGaugeInterval:  poolGaugeInterval,
		},
		LoggingMiddleware,
	)
	if err != nil {
		panic(err)
	}
	defer pool.Close()

	client := NewMyServiceClient(pool)
	if _, err = client.MyEndpoint(context.Background(), &MyEndpointRequest{}); err != nil {
		panic(err)
	}
}
