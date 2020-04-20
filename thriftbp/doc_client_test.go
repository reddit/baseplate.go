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
	remoteAddr    = "host:port"
	socketTimeout = time.Millisecond * 10

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

type Client interface {
	thriftbp.Client

	MyService
}

type clientImpl struct {
	thriftbp.Client
	MyService
}

func newClient(pool thriftbp.ClientPool) (Client, error) {
	client, err := pool.GetClient()
	if err != nil {
		return nil, err
	}
	return &clientImpl{
		MyService: NewMyServiceClient(client),
		Client:    client,
	}, nil
}

func callEndpoint(ctx context.Context, pool thriftbp.ClientPool) (*MyEndpointResponse, error) {
	client, err := newClient(pool)
	if err != nil {
		return nil, err
	}
	defer pool.ReleaseClient(client)
	return client.MyEndpoint(ctx, &MyEndpointRequest{})
}

func LoggingMiddleware(next thrift.TClient) thrift.TClient {
	return thriftbp.WrappedTClient{
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
			SocketTimeout:      socketTimeout,
			ReportPoolStats:    true,
			PoolGaugeInterval:  poolGaugeInterval,
		},
		clientTTL,
		LoggingMiddleware,
	)
	if err != nil {
		panic(err)
	}

	if _, err = callEndpoint(context.Background(), pool); err != nil {
		panic(err)
	}
}
