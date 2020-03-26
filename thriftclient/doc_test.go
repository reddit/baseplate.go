package thriftclient_test

import (
	"context"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/reddit/baseplate.go/thriftclient"
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
	thriftclient.Client

	MyService
}

type clientImpl struct {
	thriftclient.Client
	MyService
}

func newClient(pool thriftclient.ClientPool) (Client, error) {
	client, err := pool.GetClient()
	if err != nil {
		return nil, err
	}
	return &clientImpl{
		MyService: NewMyServiceClient(client),
		Client:    client,
	}, nil
}

func callEndpoint(ctx context.Context, pool thriftclient.ClientPool) (*MyEndpointResponse, error) {
	client, err := newClient(pool)
	if err != nil {
		return nil, err
	}
	defer pool.ReleaseClient(client)
	return client.MyEndpoint(ctx, &MyEndpointRequest{})
}

// This example demonstrates a typical use case of thriftclient pool in
// microservice code.
func Example() {
	pool, err := thriftclient.NewTTLClientPool(clientTTL, thriftclient.ClientPoolConfig{
		ServiceSlug:        "my-service",
		Addr:               remoteAddr,
		InitialConnections: initialConnections,
		MaxConnections:     maxConnections,
		SocketTimeout:      socketTimeout,
		ReportPoolStats:    true,
		PoolGaugeInterval:  poolGaugeInterval,
	})
	if err != nil {
		panic(err)
	}

	if _, err = callEndpoint(context.Background(), pool); err != nil {
		panic(err)
	}
}
