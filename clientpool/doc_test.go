package clientpool_test

import (
	"context"
	"errors"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/reddit/baseplate.go/clientpool"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/metricsbp"
	"github.com/reddit/baseplate.go/thriftclient"
)

// In real code these should be coming from either config file or flags instead.
const (
	remoteAddr    = "host:port"
	socketTimeout = time.Millisecond * 10

	minConnections = 50
	maxConnections = 100

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
	clientpool.Client

	MyService
}

type clientImpl struct {
	MyService

	*clientpool.TTLClient
}

func newClient(addr string) (Client, error) {
	trans, err := thrift.NewTSocketTimeout(addr, socketTimeout)
	if err != nil {
		return nil, err
	}
	err = trans.Open()
	if err != nil {
		return nil, err
	}
	protoFactory := thrift.NewTHeaderProtocolFactory()
	client := NewMyServiceClient(
		thriftclient.NewMonitoredClientFromFactory(trans, protoFactory),
	)
	return &clientImpl{
		MyService: client,
		TTLClient: clientpool.NewTTLClient(trans, clientTTL),
	}, nil
}

func reportPoolStats(ctx context.Context, pool clientpool.Pool) {
	gauge := metricsbp.M.Gauge("pool-active-connections")
	ticker := time.NewTicker(poolGaugeInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			gauge.Set(float64(pool.NumActiveClients()))
		}
	}
}

func callEndpoint(ctx context.Context, pool clientpool.Pool) (*MyEndpointResponse, error) {
	c, err := pool.Get()
	if err != nil {
		if errors.Is(err, clientpool.ErrExhausted) {
			metricsbp.M.Counter("pool-exhausted").Add(1)
		}
		return nil, err
	}
	defer func() {
		if err := pool.Release(c); err != nil {
			log.Errorw("Failed to release client back to pool", "err", err)
			metricsbp.M.Counter("pool-release-error").Add(1)
		}
	}()

	client := c.(Client)
	return client.MyEndpoint(ctx, &MyEndpointRequest{})
}

// This example demonstrates a typical use case of clientpool in microservice
// code.
func Example() {
	pool, err := clientpool.NewChannelPool(
		minConnections,
		maxConnections,
		func() (clientpool.Client, error) {
			return newClient(remoteAddr)
		},
	)
	if err != nil {
		panic(err)
	}
	go reportPoolStats(metricsbp.M.Ctx(), pool)

	callEndpoint(context.Background(), pool)
}
