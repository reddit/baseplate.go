package thriftbp_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/avast/retry-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/reddit/baseplate.go/internal/headerbp"

	"github.com/reddit/baseplate.go"
	"github.com/reddit/baseplate.go/ecinterface"
	"github.com/reddit/baseplate.go/internal/faults"
	baseplatethrift "github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
	"github.com/reddit/baseplate.go/internal/prometheusbpint/spectest"
	"github.com/reddit/baseplate.go/prometheusbp"
	"github.com/reddit/baseplate.go/retrybp"
	"github.com/reddit/baseplate.go/thriftbp"
	"github.com/reddit/baseplate.go/thriftbp/thrifttest"
	"github.com/reddit/baseplate.go/transport"
)

const (
	service = "testService"
	address = "testService.testNamespace.svc.cluster.local:12345"
	method  = "testMethod"
)

func initClients(ecImpl ecinterface.Interface) (*thrifttest.MockClient, *thrifttest.RecordedClient, thrift.TClient) {
	if ecImpl == nil {
		ecImpl = ecinterface.Mock()
	}
	mock := &thrifttest.MockClient{FailUnregisteredMethods: true}
	recorder := thrifttest.NewRecordedClient(mock)
	client := thrift.WrapClient(
		recorder,
		thriftbp.BaseplateDefaultClientMiddlewares(
			thriftbp.DefaultClientMiddlewareArgs{
				EdgeContextImpl: ecImpl,
				ServiceSlug:     service,
				Address:         address,
			},
		)...,
	)
	return mock, recorder, client
}

func TestForwardEdgeRequestContext(t *testing.T) {
	const expectedHeader = "dummy-edge-context"

	impl := ecinterface.Mock()
	ctx, _ := impl.HeaderToContext(context.Background(), expectedHeader)
	ctx = thriftbp.AttachEdgeRequestContext(ctx, impl)

	mock, recorder, client := initClients(impl)
	mock.AddMockCall(
		method,
		func(ctx context.Context, args, result thrift.TStruct) (meta thrift.ResponseMeta, err error) {
			return
		},
	)

	if _, err := client.Call(ctx, method, nil, nil); err != nil {
		t.Fatal(err)
	}

	if len(recorder.Calls()) != 1 {
		t.Fatalf("wrong number of calls: %d", len(recorder.Calls()))
	}

	ctx = recorder.Calls()[0].Ctx
	headerInWriteHeaderList(ctx, t, transport.HeaderEdgeRequest)

	header, ok := thrift.GetHeader(ctx, transport.HeaderEdgeRequest)
	if !ok {
		t.Fatal("header not set")
	}
	if header != expectedHeader {
		t.Errorf("header mismatch, expected %q, got %q", expectedHeader, header)
	}
}

func TestForwardEdgeRequestContextNotSet(t *testing.T) {
	mock, recorder, client := initClients(ecinterface.Mock())
	mock.AddMockCall(
		method,
		func(ctx context.Context, args, result thrift.TStruct) (meta thrift.ResponseMeta, err error) {
			return
		},
	)

	if _, err := client.Call(context.Background(), method, nil, nil); err != nil {
		t.Fatal(err)
	}

	if len(recorder.Calls()) != 1 {
		t.Fatalf("wrong number of calls: %d", len(recorder.Calls()))
	}

	ctx := recorder.Calls()[0].Ctx
	_, ok := thrift.GetHeader(ctx, transport.HeaderEdgeRequest)
	if ok {
		t.Fatal("edge request header should not be set")
	}
}

func TestSetDeadlineBudget(t *testing.T) {
	mock, recorder, client := initClients(nil)
	mock.AddMockCall(
		method,
		func(ctx context.Context, args, result thrift.TStruct) (meta thrift.ResponseMeta, err error) {
			return
		},
	)

	t.Run(
		"already-passed",
		func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			_, err := client.Call(ctx, method, nil, nil)
			if err == nil {
				t.Error("Expect error when ctx is already cancelled, got nil")
			}

			if len(recorder.Calls()) != 0 {
				t.Fatalf("Wrong number of calls: %d", len(recorder.Calls()))
			}
		},
	)

	t.Run(
		"less-than-1ms",
		func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond-1)
			defer cancel()

			if _, err := client.Call(ctx, method, nil, nil); err != nil {
				t.Fatal(err)
			}

			if len(recorder.Calls()) != 1 {
				t.Fatalf("Wrong number of calls: %d", len(recorder.Calls()))
			}

			ctx = recorder.Calls()[0].Ctx
			v, ok := thrift.GetHeader(ctx, transport.HeaderDeadlineBudget)
			if !ok {
				t.Fatalf("%s header not set", transport.HeaderDeadlineBudget)
			}
			if v != "1" {
				t.Errorf(
					"Expected 1 in header %s, got %q",
					transport.HeaderDeadlineBudget,
					v,
				)
			}

			headerInWriteHeaderList(ctx, t, transport.HeaderDeadlineBudget)
		},
	)
}

const retryTestTimeout = 10 * time.Millisecond

type BaseplateService struct {
	server baseplate.Server
}

func (srv BaseplateService) IsHealthy(ctx context.Context, _ *baseplatethrift.IsHealthyRequest) (r bool, err error) {
	srv.server.Close()
	time.Sleep(retryTestTimeout)
	return true, nil
}

type counter struct {
	count int
}

func (c *counter) incr() {
	c.count++
}

func (c *counter) onRetry(n uint, err error) {
	c.incr()
}

func TestRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := newSecretsStore(t)

	c := &counter{}
	handler := BaseplateService{}
	processor := baseplatethrift.NewBaseplateServiceV2Processor(&handler)
	server, err := thrifttest.NewBaseplateServer(thrifttest.ServerConfig{
		Processor:   processor,
		SecretStore: store,
		ClientConfig: thriftbp.ClientPoolConfig{
			EdgeContextImpl: ecinterface.Mock(),
			DefaultRetryOptions: []retry.Option{
				retry.Attempts(2),
				retrybp.Filters(retrybp.NetworkErrorFilter),
				retry.OnRetry(c.onRetry),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	handler.server = server
	server.Start(ctx)

	client := baseplatethrift.NewBaseplateServiceV2Client(server.ClientPool.TClient())
	ctx, cancel = context.WithTimeout(ctx, retryTestTimeout)
	defer cancel()
	_, err = client.IsHealthy(
		ctx,
		&baseplatethrift.IsHealthyRequest{
			Probe: baseplatethrift.IsHealthyProbePtr(baseplatethrift.IsHealthyProbe_READINESS),
		},
	)
	t.Logf("error: %v", err)
	if err == nil {
		t.Error("expected an error, got nil")
	}
	const expected = 1
	if c.count != expected {
		t.Errorf("expected middleware to trigger a retry %d times, got %d", expected, c.count)
	}
}

func TestSetClientName(t *testing.T) {
	const header = transport.HeaderUserAgent

	initClientsForUA := func(ua string) (*thrifttest.RecordedClient, thrift.TClient) {
		ecImpl := ecinterface.Mock()
		mock := &thrifttest.MockClient{FailUnregisteredMethods: true}
		mock.AddMockCall(
			method,
			func(ctx context.Context, args, result thrift.TStruct) (meta thrift.ResponseMeta, err error) {
				return
			},
		)

		recorder := thrifttest.NewRecordedClient(mock)
		client := thrift.WrapClient(
			recorder,
			thriftbp.BaseplateDefaultClientMiddlewares(
				thriftbp.DefaultClientMiddlewareArgs{
					EdgeContextImpl: ecImpl,
					ServiceSlug:     service,
					ClientName:      ua,
				},
			)...,
		)
		return recorder, client
	}

	t.Run(
		"unset",
		func(t *testing.T) {
			const ua = ""
			recorder, client := initClientsForUA(ua)

			_, err := client.Call(context.Background(), method, nil, nil)
			if err != nil {
				t.Fatal(err)
			}

			if len(recorder.Calls()) != 1 {
				t.Fatalf("Wrong number of calls: %d", len(recorder.Calls()))
			}

			ctx := recorder.Calls()[0].Ctx
			headers := thrift.GetWriteHeaderList(ctx)
			for _, h := range headers {
				if h == header {
					t.Errorf("Did not expect header %q in write header list", header)
				}
			}
			v, ok := thrift.GetHeader(ctx, header)
			if ok || v != "" {
				t.Errorf("Did not expect header %q, got %q, %v", header, v, ok)
			}
		},
	)

	t.Run(
		"set",
		func(t *testing.T) {
			const ua = "foo"
			recorder, client := initClientsForUA(ua)

			_, err := client.Call(context.Background(), method, nil, nil)
			if err != nil {
				t.Fatal(err)
			}

			if len(recorder.Calls()) != 1 {
				t.Fatalf("Wrong number of calls: %d", len(recorder.Calls()))
			}

			ctx := recorder.Calls()[0].Ctx
			headerInWriteHeaderList(ctx, t, header)
			if v, ok := thrift.GetHeader(ctx, header); v != ua {
				t.Errorf("Expected header %q to be %q, got %q, %v", header, ua, v, ok)
			}
		},
	)
}

const (
	methodIsHealthy = "is_healthy"
)

const (
	methodLabel                  = "thrift_method"
	successLabel                 = "thrift_success"
	exceptionLabel               = "thrift_exception_type"
	baseplateStatusLabel         = "thrift_baseplate_status"
	baseplateStatusCodeLabel     = "thrift_baseplate_status_code"
	remoteServiceClientNameLabel = "thrift_client_name"
)

func TestPrometheusClientMiddleware(t *testing.T) {
	testCases := []struct {
		name          string
		wantErr       error
		wantFail      bool
		exceptionType string
	}{
		{
			name:          "error",
			wantErr:       errors.New("test"),
			wantFail:      true,
			exceptionType: "thrift.tApplicationException",
		},
		{
			name:          "success",
			wantErr:       nil,
			wantFail:      false,
			exceptionType: "",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {

			latencyLabels := prometheus.Labels{
				methodLabel:                  methodIsHealthy,
				successLabel:                 prometheusbp.BoolString(!tt.wantFail),
				remoteServiceClientNameLabel: tt.name,
			}

			totalRequestLabels := prometheus.Labels{
				methodLabel:                  methodIsHealthy,
				successLabel:                 prometheusbp.BoolString(!tt.wantFail),
				exceptionLabel:               tt.exceptionType,
				baseplateStatusCodeLabel:     "",
				baseplateStatusLabel:         "",
				remoteServiceClientNameLabel: tt.name,
			}

			activeRequestLabels := prometheus.Labels{
				methodLabel:                  methodIsHealthy,
				remoteServiceClientNameLabel: tt.name,
			}

			defer thriftbp.PrometheusClientMetricsTest(t, latencyLabels, totalRequestLabels, activeRequestLabels).CheckMetrics()
			defer spectest.ValidateSpec(t, "thrift", "client")

			ctx := context.Background()
			handler := mockBaseplateService{fail: tt.wantFail, err: tt.wantErr}
			client := setupFake(ctx, t, handler, tt.name)
			bpClient := baseplatethrift.NewBaseplateServiceV2Client(client.TClient())
			result, err := bpClient.IsHealthy(
				ctx,
				&baseplatethrift.IsHealthyRequest{
					Probe: baseplatethrift.IsHealthyProbePtr(baseplatethrift.IsHealthyProbe_READINESS),
				},
			)
			if tt.wantErr != nil && err == nil {
				t.Error("expected an error, got nil")
			} else if tt.wantErr == nil && err != nil {
				t.Errorf("expected no error, got %v", err)
			}

			if result == tt.wantFail {
				t.Errorf("result mismatch, expected %v, got %v", tt.wantFail, result)
			}
		})
	}
}

func TestFaultInjectionClientMiddleware(t *testing.T) {
	testCases := []struct {
		name string

		faultServerAddrHeader      string
		faultServerMethodHeader    string
		faultDelayMsHeader         string
		faultDelayPercentageHeader string
		faultAbortCodeHeader       string
		faultAbortMessageHeader    string
		faultAbortPercentageHeader string

		wantErr error
	}{
		{
			name:    "no fault specified",
			wantErr: nil,
		},
		{
			name: "abort",

			faultServerAddrHeader:   "testService.testNamespace",
			faultServerMethodHeader: "testMethod",
			faultAbortCodeHeader:    "1", // NOT_OPEN
			faultAbortMessageHeader: "test fault",

			wantErr: thrift.NewTTransportException(1, "test fault"),
		},
		{
			name: "service does not match",

			faultServerAddrHeader:   "foo",
			faultServerMethodHeader: "testMethod",
			faultAbortCodeHeader:    "1", // NOT_OPEN
			faultAbortMessageHeader: "test fault",

			wantErr: nil,
		},
		{
			name: "method does not match",

			faultServerAddrHeader:   "testService.testNamespace",
			faultServerMethodHeader: "foo",
			faultAbortCodeHeader:    "1", // NOT_OPEN
			faultAbortMessageHeader: "test fault",

			wantErr: nil,
		},
		{
			name: "less than min abort code",

			faultServerAddrHeader:   "testService.testNamespace",
			faultServerMethodHeader: "testMethod",
			faultAbortCodeHeader:    "-1",
			faultAbortMessageHeader: "test fault",

			wantErr: nil,
		},
		{
			name: "greater than max abort code",

			faultServerAddrHeader:   "testService.testNamespace",
			faultServerMethodHeader: "testMethod",
			faultAbortCodeHeader:    "5",
			faultAbortMessageHeader: "test fault",

			wantErr: nil,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			impl := ecinterface.Mock()
			ctx := context.Background()

			// Set the headers on the context as if the incoming
			// request contained them. Setting via
			// thrift.AddClientHeader would result in the headers
			// being blocked by the headerbp middleware, as new
			// headers are not allowed to be propagated.
			headers := headerbp.NewIncomingHeaders(
				headerbp.WithThriftService("", ""),
			)

			if tt.faultServerAddrHeader != "" {
				headers.RecordHeader(faults.FaultServerAddressHeader, tt.faultServerAddrHeader)
			}
			if tt.faultServerMethodHeader != "" {
				headers.RecordHeader(faults.FaultServerMethodHeader, tt.faultServerMethodHeader)
			}
			if tt.faultDelayMsHeader != "" {
				headers.RecordHeader(faults.FaultDelayMsHeader, tt.faultDelayMsHeader)
			}
			if tt.faultDelayPercentageHeader != "" {
				headers.RecordHeader(faults.FaultDelayPercentageHeader, tt.faultDelayPercentageHeader)
			}
			if tt.faultAbortCodeHeader != "" {
				headers.RecordHeader(faults.FaultAbortCodeHeader, tt.faultAbortCodeHeader)
			}
			if tt.faultAbortMessageHeader != "" {
				headers.RecordHeader(faults.FaultAbortMessageHeader, tt.faultAbortMessageHeader)
			}
			if tt.faultAbortPercentageHeader != "" {
				headers.RecordHeader(faults.FaultAbortPercentageHeader, tt.faultAbortPercentageHeader)
			}

			ctx = headers.SetOnContext(ctx)

			mock, _, client := initClients(impl)
			mock.AddMockCall(
				method,
				func(ctx context.Context, args, result thrift.TStruct) (meta thrift.ResponseMeta, err error) {
					return
				},
			)

			_, err := client.Call(ctx, method, nil, nil)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tt.wantErr != nil && err == nil {
				t.Fatal("expected an error, got nil")
			}
			if err != nil && err.Error() != tt.wantErr.Error() {
				t.Fatalf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}

type mockBaseplateService struct {
	fail bool
	err  error
}

func (srv mockBaseplateService) IsHealthy(ctx context.Context, req *baseplatethrift.IsHealthyRequest) (r bool, err error) {
	return !srv.fail, srv.err
}

func setupFake(ctx context.Context, t *testing.T, handler baseplatethrift.BaseplateServiceV2, slug string) thriftbp.ClientPool {
	srv, err := thrifttest.NewBaseplateServer(thrifttest.ServerConfig{
		Processor: baseplatethrift.NewBaseplateServiceV2Processor(handler),
		ClientConfig: thriftbp.ClientPoolConfig{
			ServiceSlug: slug,
		},
	})
	if err != nil {
		t.Fatalf("SETUP: Setting up baseplate server: %s", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)

	// Shut down the start goroutine when the test completes
	srv.Start(ctx)

	// The thrift server doesn't shut down cleanly, so we have to close it in a goroutine :(
	t.Cleanup(func() { go srv.Close() })

	return srv.ClientPool
}
