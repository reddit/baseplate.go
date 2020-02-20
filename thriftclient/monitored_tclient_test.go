package thriftclient_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/reddit/baseplate.go/mqsend"
	"github.com/reddit/baseplate.go/runtimebp"
	"github.com/reddit/baseplate.go/thriftclient"
	"github.com/reddit/baseplate.go/tracing"
)

const (
	method = "foo"
)

func initClients() (*thriftclient.MockClient, *thriftclient.RecordedClient, *thriftclient.MonitoredClient) {
	mock := &thriftclient.MockClient{}
	recorder := thriftclient.NewRecordedClient(mock)
	client := &thriftclient.MonitoredClient{Client: recorder}
	return mock, recorder, client
}

func initServerSpan(ctx context.Context) (context.Context, *tracing.Tracer) {
	ip, _ := runtimebp.GetFirstIPv4()
	tracer := &tracing.Tracer{
		SampleRate: 1.0,
		Endpoint: tracing.ZipkinEndpointInfo{
			ServiceName: "test-service",
			IPv4:        ip,
		},
	}
	tracer.Recorder = mqsend.OpenMockMessageQueue(mqsend.MessageQueueConfig{
		MaxQueueSize:   100,
		MaxMessageSize: 1024,
	})

	ctx, span := tracing.CreateServerSpanForContext(ctx, tracer, "test-service")
	span.SetDebug(true)
	return ctx, tracer
}

func initLocalSpan(ctx context.Context) (context.Context, *tracing.Tracer) {
	ctx, tracer := initServerSpan(ctx)
	span := tracing.GetServerSpan(ctx)
	if span == nil {
		panic("server span was nill")
	}
	ctx, _ = span.CreateLocalChildForContext(ctx, "local-test", "")
	return ctx, tracer
}

func drainTracer(t *tracing.Tracer) ([]byte, error) {
	mmq := t.Recorder.(*mqsend.MockMessageQueue)
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	return mmq.Receive(ctx)
}

func TestMonitoredClientNilSpan(t *testing.T) {
	t.Parallel()

	_, recorder, client := initClients()
	if err := client.Call(context.Background(), method, nil, nil); err != nil {
		t.Fatal(err)
	}
	call := recorder.Calls()[0]
	span := tracing.GetActiveSpan(call.Ctx)
	if span != nil {
		t.Errorf("expected nil span, got %#v", span)
	}
	if call.Method != method {
		t.Errorf("method mismatch, expected %q, got %q", method, call.Method)
	}
}

func TestMonitoredClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		call          thriftclient.MockCall
		errorExpected bool
		initSpan      func(context.Context) (context.Context, *tracing.Tracer)
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
	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				t.Parallel()

				mock, recorder, client := initClients()
				mock.AddMockCall(method, c.call)

				ctx, tracer := c.initSpan(context.Background())
				if err := client.Call(ctx, method, nil, nil); !c.errorExpected && err != nil {
					t.Fatal(err)
				} else if c.errorExpected && err == nil {
					t.Fatal("expected an error, got nil")
				}
				call := recorder.Calls()[0]
				span := tracing.GetActiveSpan(call.Ctx)
				if span == nil {
					t.Fatal("span was nil")
				}
				if span.Name() != method {
					t.Errorf("span name mismatch, expected %q, got %q", method, span.Name())
				}
				if span.SpanType() != tracing.SpanTypeClient {
					t.Errorf("span type mismatch, expected %s, got %s", tracing.SpanTypeClient, span.SpanType())
				}
				if call.Method != method {
					t.Errorf("method mismatch, expected %q, got %q", method, call.Method)
				}

				msg, err := drainTracer(tracer)
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
