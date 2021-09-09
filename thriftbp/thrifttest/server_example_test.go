package thrifttest_test

import (
	"context"
	"testing"

	baseplatethrift "github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
	"github.com/reddit/baseplate.go/secrets"
	"github.com/reddit/baseplate.go/thriftbp/thrifttest"
)

type mockedBaseplateService struct {
	Fail bool
	Err  error
}

func (srv mockedBaseplateService) IsHealthy(ctx context.Context, req *baseplatethrift.IsHealthyRequest) (r bool, err error) {
	return !srv.Fail, srv.Err
}

// In real test this function needs to be named TestService instead,
// but doing that will break this example.
func ServiceTest(t *testing.T) {
	// Initialize this properly in a real test,
	// usually via secrets.NewTestSecrets.
	var store *secrets.Store

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	processor := baseplatethrift.NewBaseplateServiceV2Processor(mockedBaseplateService{})
	server, err := thrifttest.NewBaseplateServer(thrifttest.ServerConfig{
		Processor:   processor,
		SecretStore: store,
	})
	if err != nil {
		t.Fatal(err)
	}
	// cancelling the context will close the server.
	server.Start(ctx)

	client := baseplatethrift.NewBaseplateServiceV2Client(server.ClientPool.TClient())
	got, err := client.IsHealthy(ctx, &baseplatethrift.IsHealthyRequest{
		Probe: baseplatethrift.IsHealthyProbePtr(baseplatethrift.IsHealthyProbe_READINESS),
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	const want = true
	if got != want {
		t.Errorf("success mismatch, want %v, got %v", want, got)
	}
}

// This example demonstrates how to write an unit test with mocked thrift server
// using NewBaseplateServer.
func ExampleNewBaseplateServer() {
	// The real example is in ServiceTest function.
}
