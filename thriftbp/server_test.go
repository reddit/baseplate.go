package thriftbp_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/reddit/baseplate.go/headerbp"

	"github.com/reddit/baseplate.go/ecinterface"
	baseplatethrift "github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
	"github.com/reddit/baseplate.go/thriftbp"
	"github.com/reddit/baseplate.go/thriftbp/thrifttest"
)

type headerPropagationVerificationServic struct {
	want      map[string]string
	wantUnset []string

	client func() baseplatethrift.BaseplateServiceV2
}

func (s *headerPropagationVerificationServic) IsHealthy(ctx context.Context, _ *baseplatethrift.IsHealthyRequest) (bool, error) {
	var errs []error
	got := make(map[string]string, len(s.want))
	for k := range s.want {
		got[k], _ = thrift.GetHeader(ctx, k)
	}
	if diff := cmp.Diff(s.want, got, cmpopts.EquateEmpty()); diff != "" {
		errs = append(errs, fmt.Errorf("header mismatch (-want +got): %s", diff))
	}

	var unwantedHeaders []string
	for _, k := range s.wantUnset {
		if _, ok := thrift.GetHeader(ctx, k); ok {
			unwantedHeaders = append(unwantedHeaders, k)
		}
	}
	if len(unwantedHeaders) > 0 {
		errs = append(errs, fmt.Errorf("unwanted headers: %v", unwantedHeaders))
	}

	if err := errors.Join(errs...); err != nil {
		return false, err
	}

	outgoingCtx := setHeader(ctx, "x-bp-test", "bar")
	if _, err := s.client().IsHealthy(outgoingCtx, &baseplatethrift.IsHealthyRequest{}); !errors.Is(err, headerbp.ErrNewInternalHeaderNotAllowed) {
		return false, fmt.Errorf("error mismatch, want %v, got %v", headerbp.ErrNewInternalHeaderNotAllowed, err)
	}
	return true, nil
}

type echoService struct{}

func (s *echoService) IsHealthy(ctx context.Context, req *baseplatethrift.IsHealthyRequest) (bool, error) {
	return true, nil
}

func TestHeaderPropagation(t *testing.T) {
	store := newSecretsStore(t)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	ecImpl := ecinterface.Mock()

	downstreamProcessor := baseplatethrift.NewBaseplateServiceV2Processor(&echoService{})
	downstreamServer, err := thrifttest.NewBaseplateServer(thrifttest.ServerConfig{
		Processor:       downstreamProcessor,
		SecretStore:     store,
		EdgeContextImpl: ecImpl,
	})
	if err != nil {
		t.Fatal(err)
	}
	downstreamServer.Start(ctx)
	time.Sleep(100 * time.Millisecond) // wait for the server to start

	originProcessor := baseplatethrift.NewBaseplateServiceV2Processor(&headerPropagationVerificationServic{
		want: map[string]string{
			"x-bp-test": "foo",
		},
		client: func() baseplatethrift.BaseplateServiceV2 {
			return baseplatethrift.NewBaseplateServiceV2Client(downstreamServer.ClientPool.TClient())
		},
	})
	server, err := thrifttest.NewBaseplateServer(thrifttest.ServerConfig{
		Processor:   originProcessor,
		SecretStore: store,
	})
	if err != nil {
		t.Fatal(err)
	}
	server.Start(ctx)
	time.Sleep(100 * time.Millisecond) // wait for the server to start

	clientCfg := thriftbp.ClientPoolConfig{
		ServiceSlug:        thrifttest.DefaultServiceSlug,
		Addr:               server.Baseplate().GetConfig().Addr,
		InitialConnections: thrifttest.InitialClientConnections,
		MaxConnections:     thrifttest.DefaultClientMaxConnections,
		ConnectTimeout:     thrifttest.DefaultClientConnectTimeout,
		SocketTimeout:      thrifttest.DefaultClientSocketTimeout,
		EdgeContextImpl:    ecImpl,
		ClientName:         "header-check",
	}
	// we have to use a custom pool to avoid using the default middleware which will block baseplate headers
	pool, err := thriftbp.NewCustomClientPoolWithContext(
		ctx,
		clientCfg,
		thriftbp.SingleAddressGenerator(clientCfg.Addr),
		thrift.NewTHeaderProtocolFactoryConf(clientCfg.ToTConfiguration()),
	)
	if err != nil {
		server.Close()
		t.Fatalf("error creating client pool: %v", err)
	}
	client := baseplatethrift.NewBaseplateServiceV2Client(pool.TClient())
	ctx = setHeader(ctx, "x-bp-test", "foo")
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

func setHeader(ctx context.Context, key, value string) context.Context {
	ctx = thrift.SetHeader(ctx, key, value)
	return thrift.SetWriteHeaderList(ctx, append(thrift.GetWriteHeaderList(ctx), key))
}
