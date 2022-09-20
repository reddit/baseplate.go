package thriftbp_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/avast/retry-go"
	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/reddit/baseplate.go"
	"github.com/reddit/baseplate.go/ecinterface"
	baseplatethrift "github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
	"github.com/reddit/baseplate.go/internal/prometheusbpint/spectest"
	"github.com/reddit/baseplate.go/mqsend"
	"github.com/reddit/baseplate.go/prometheusbp"
	"github.com/reddit/baseplate.go/retrybp"
	"github.com/reddit/baseplate.go/thriftbp"
	"github.com/reddit/baseplate.go/thriftbp/thrifttest"
	"github.com/reddit/baseplate.go/tracing"
	"github.com/reddit/baseplate.go/transport"
)

const (
	service = "testService"
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
			},
		)...,
	)
	return mock, recorder, client
}

func initServerSpan(ctx context.Context, t *testing.T) (context.Context, *mqsend.MockMessageQueue) {
	t.Helper()

	recorder := mqsend.OpenMockMessageQueue(mqsend.MessageQueueConfig{
		MaxQueueSize:   100,
		MaxMessageSize: 1024,
	})
	tracing.InitGlobalTracer(tracing.Config{
		SampleRate:               1.0,
		TestOnlyMockMessageQueue: recorder,
	})

	span, ctx := opentracing.StartSpanFromContext(
		ctx,
		"test-service",
		tracing.SpanTypeOption{Type: tracing.SpanTypeServer},
	)
	tracing.AsSpan(span).SetDebug(true)
	return ctx, recorder
}

func initLocalSpan(ctx context.Context, t *testing.T) (context.Context, *mqsend.MockMessageQueue) {
	t.Helper()

	ctx, recorder := initServerSpan(ctx, t)
	span := opentracing.SpanFromContext(ctx)
	if span == nil {
		t.Fatal("server span was nill")
	}
	_, ctx = opentracing.StartSpanFromContext(
		ctx,
		"local-test",
		tracing.SpanTypeOption{
			Type: tracing.SpanTypeLocal,
		},
	)
	return ctx, recorder
}

func drainRecorder(recorder *mqsend.MockMessageQueue) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	return recorder.Receive(ctx)
}

func TestWrapMonitoredClient(t *testing.T) {
	cases := []struct {
		name          string
		call          thrifttest.MockCall
		errorExpected bool
		initSpan      func(context.Context, *testing.T) (context.Context, *mqsend.MockMessageQueue)
	}{
		{
			name: "server span: success",
			call: func(ctx context.Context, args, result thrift.TStruct) (meta thrift.ResponseMeta, err error) {
				return
			},
			errorExpected: false,
			initSpan:      initServerSpan,
		},
		{
			name: "server span: error",
			call: func(ctx context.Context, args, result thrift.TStruct) (thrift.ResponseMeta, error) {
				return thrift.ResponseMeta{}, errors.New("test error")
			},
			errorExpected: true,
			initSpan:      initServerSpan,
		},
		{
			name: "local span: success",
			call: func(ctx context.Context, args, result thrift.TStruct) (thrift.ResponseMeta, error) {
				return thrift.ResponseMeta{}, nil
			},
			errorExpected: false,
			initSpan:      initLocalSpan,
		},
		{
			name: "local span: error",
			call: func(ctx context.Context, args, result thrift.TStruct) (thrift.ResponseMeta, error) {
				return thrift.ResponseMeta{}, errors.New("test error")
			},
			errorExpected: true,
			initSpan:      initLocalSpan,
		},
	}
	for _, c := range cases {
		t.Run(
			c.name,
			func(t *testing.T) {
				defer func() {
					tracing.CloseTracer()
					tracing.InitGlobalTracer(tracing.Config{})
				}()

				mock, recorder, client := initClients(nil)
				mock.AddMockCall(method, c.call)

				ctx, mmq := c.initSpan(context.Background(), t)
				if _, err := client.Call(ctx, method, nil, nil); !c.errorExpected && err != nil {
					t.Fatal(err)
				} else if c.errorExpected && err == nil {
					t.Fatal("expected an error, got nil")
				}
				call := recorder.Calls()[0]
				s := opentracing.SpanFromContext(call.Ctx)
				if s == nil {
					t.Fatal("span was nil")
				}
				spanName := service + "." + method
				span := tracing.AsSpan(s)
				if span.Name() != spanName {
					t.Errorf("span name mismatch, expected %q, got %q", spanName, span.Name())
				}
				if span.SpanType() != tracing.SpanTypeClient {
					t.Errorf("span type mismatch, expected %s, got %s", tracing.SpanTypeClient, span.SpanType())
				}
				if call.Method != method {
					t.Errorf("method mismatch, expected %q, got %q", method, call.Method)
				}

				msg, err := drainRecorder(mmq)
				if err != nil {
					t.Fatal(err)
				}

				var trace tracing.ZipkinSpan
				err = json.Unmarshal(msg, &trace)
				if err != nil {
					t.Fatal(err)
				}
				hasError := false
				for _, annotation := range trace.BinaryAnnotations {
					if annotation.Key == "error" {
						hasError = true
					}
				}
				if !c.errorExpected && hasError {
					t.Error("error binary annotation present")
				} else if c.errorExpected && !hasError {
					t.Error("error binary annotation not present")
				}
			},
		)
	}
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
	defer store.Close()

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
				remoteServiceClientNameLabel: thrifttest.DefaultServiceSlug,
			}

			totalRequestLabels := prometheus.Labels{
				methodLabel:                  methodIsHealthy,
				successLabel:                 prometheusbp.BoolString(!tt.wantFail),
				exceptionLabel:               tt.exceptionType,
				baseplateStatusCodeLabel:     "",
				baseplateStatusLabel:         "",
				remoteServiceClientNameLabel: thrifttest.DefaultServiceSlug,
			}

			activeRequestLabels := prometheus.Labels{
				methodLabel:                  methodIsHealthy,
				remoteServiceClientNameLabel: thrifttest.DefaultServiceSlug,
			}

			defer thriftbp.PrometheusClientMetricsTest(t, latencyLabels, totalRequestLabels, activeRequestLabels).CheckMetrics()
			defer spectest.ValidateSpec(t, "thrift", "client")

			ctx := context.Background()
			handler := mockBaseplateService{fail: tt.wantFail, err: tt.wantErr}
			client := setupFake(ctx, t, handler)
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

type mockBaseplateService struct {
	fail bool
	err  error
}

func (srv mockBaseplateService) IsHealthy(ctx context.Context, req *baseplatethrift.IsHealthyRequest) (r bool, err error) {
	return !srv.fail, srv.err
}

func setupFake(ctx context.Context, t *testing.T, handler baseplatethrift.BaseplateServiceV2) thriftbp.ClientPool {
	srv, err := thrifttest.NewBaseplateServer(thrifttest.ServerConfig{
		Processor: baseplatethrift.NewBaseplateServiceV2Processor(handler),
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
