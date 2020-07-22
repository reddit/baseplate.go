package thriftbp_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/avast/retry-go"
	opentracing "github.com/opentracing/opentracing-go"

	baseplate "github.com/reddit/baseplate.go"
	"github.com/reddit/baseplate.go/edgecontext"
	baseplatethrift "github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
	"github.com/reddit/baseplate.go/mqsend"
	"github.com/reddit/baseplate.go/retrybp"
	"github.com/reddit/baseplate.go/thriftbp"
	"github.com/reddit/baseplate.go/thriftbp/thrifttest"
	"github.com/reddit/baseplate.go/tracing"
)

const (
	service = "testService"
	method  = "testMethod"
)

func initClients() (*thrifttest.MockClient, *thrifttest.RecordedClient, thrift.TClient) {
	mock := &thrifttest.MockClient{FailUnregisteredMethods: true}
	recorder := thrifttest.NewRecordedClient(mock)
	client := thrift.WrapClient(
		recorder,
		thriftbp.BaseplateDefaultClientMiddlewares(
			thriftbp.DefaultClientMiddlewareArgs{ServiceSlug: service},
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
	tracing.InitGlobalTracer(tracing.TracerConfig{
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
		tracing.LocalComponentOption{Name: ""},
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
			call: func(ctx context.Context, args, result thrift.TStruct) error {
				return nil
			},
			errorExpected: false,
			initSpan:      initServerSpan,
		},
		{
			name: "server span: error",
			call: func(ctx context.Context, args, result thrift.TStruct) error {
				return errors.New("test error")
			},
			errorExpected: true,
			initSpan:      initServerSpan,
		},
		{
			name: "local span: success",
			call: func(ctx context.Context, args, result thrift.TStruct) error {
				return nil
			},
			errorExpected: false,
			initSpan:      initLocalSpan,
		},
		{
			name: "local span: error",
			call: func(ctx context.Context, args, result thrift.TStruct) error {
				return errors.New("test error")
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
					tracing.InitGlobalTracer(tracing.TracerConfig{})
				}()

				mock, recorder, client := initClients()
				mock.AddMockCall(method, c.call)

				ctx, mmq := c.initSpan(context.Background(), t)
				if err := client.Call(ctx, method, nil, nil); !c.errorExpected && err != nil {
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
	store := newSecretsStore(t)
	defer store.Close()

	impl := edgecontext.Init(edgecontext.Config{Store: store})
	ec, err := edgecontext.FromHeader(context.Background(), headerWithValidAuth, impl)
	if err != nil {
		t.Fatal(err)
	}

	ctx := thrift.SetHeader(
		context.Background(),
		thriftbp.HeaderEdgeRequest,
		headerWithValidAuth,
	)
	ctx = thriftbp.InitializeEdgeContext(ctx, impl)

	mock, recorder, client := initClients()
	mock.AddMockCall(
		method,
		func(ctx context.Context, args, result thrift.TStruct) error {
			return nil
		},
	)

	if err := client.Call(ctx, method, nil, nil); err != nil {
		t.Fatal(err)
	}

	if len(recorder.Calls()) != 1 {
		t.Fatalf("wrong number of calls: %d", len(recorder.Calls()))
	}

	ctx = recorder.Calls()[0].Ctx
	headers := thrift.GetWriteHeaderList(ctx)
	var found bool
	for _, key := range headers {
		if key == thriftbp.HeaderEdgeRequest {
			found = true
			break
		}
	}
	if !found {
		t.Error("header not added to thrift write list")
	}

	header, ok := thrift.GetHeader(ctx, thriftbp.HeaderEdgeRequest)
	if !ok {
		t.Fatal("header not set")
	}
	if header != ec.Header() {
		t.Errorf("header mismatch, expected %q, got %q", ec.Header(), header)
	}
}

func TestForwardEdgeRequestContextNotSet(t *testing.T) {
	mock, recorder, client := initClients()
	mock.AddMockCall(
		method,
		func(ctx context.Context, args, result thrift.TStruct) error {
			return nil
		},
	)

	if err := client.Call(context.Background(), method, nil, nil); err != nil {
		t.Fatal(err)
	}

	if len(recorder.Calls()) != 1 {
		t.Fatalf("wrong number of calls: %d", len(recorder.Calls()))
	}

	ctx := recorder.Calls()[0].Ctx
	_, ok := thrift.GetHeader(ctx, thriftbp.HeaderEdgeRequest)
	if ok {
		t.Fatal("edge request header should not be set")
	}
}

func TesetSetDeadlineBudget(t *testing.T) {
	mock, recorder, client := initClients()
	mock.AddMockCall(
		method,
		func(ctx context.Context, args, result thrift.TStruct) error {
			return nil
		},
	)

	t.Run(
		"already-passed",
		func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			err := client.Call(ctx, method, nil, nil)
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

			if err := client.Call(ctx, method, nil, nil); err != nil {
				t.Fatal(err)
			}

			if len(recorder.Calls()) != 1 {
				t.Fatalf("Wrong number of calls: %d", len(recorder.Calls()))
			}

			ctx = recorder.Calls()[0].Ctx
			v, ok := thrift.GetHeader(ctx, thriftbp.HeaderDeadlineBudget)
			if !ok {
				t.Fatalf("%s header not set", thriftbp.HeaderDeadlineBudget)
			}
			if v != "1" {
				t.Errorf(
					"Expected 1 in header %s, got %q",
					thriftbp.HeaderDeadlineBudget,
					v,
				)
			}
		},
	)
}

type BaseplateService struct {
	Sever baseplate.Server
}

func (srv BaseplateService) IsHealthy(ctx context.Context, _ *baseplatethrift.IsHealthyRequest) (r bool, err error) {
	srv.Sever.Close()
	time.Sleep(10 * time.Millisecond)
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
	handler.Sever = server
	server.Start(ctx)

	client := baseplatethrift.NewBaseplateServiceV2Client(server.ClientPool)
	_, err = client.IsHealthy(
		ctx,
		&baseplatethrift.IsHealthyRequest{
			Probe: baseplatethrift.IsHealthyProbePtr(baseplatethrift.IsHealthyProbe_READINESS),
		},
	)
	if err == nil {
		t.Errorf("expected an error, got nil")
	}
	if c.count != 1 {
		t.Errorf("expected middleware to trigger a retry.")
	}
}
