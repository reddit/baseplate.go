package thriftclient_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	opentracing "github.com/opentracing/opentracing-go"

	"github.com/reddit/baseplate.go/mqsend"
	"github.com/reddit/baseplate.go/thriftclient"
	"github.com/reddit/baseplate.go/tracing"
)

const (
	method = "foo"
)

func initClients() (*thriftclient.MockClient, *thriftclient.RecordedClient, *thriftclient.MonitoredClient) {
	mock := &thriftclient.MockClient{FailUnregisteredMethods: true}
	recorder := thriftclient.NewRecordedClient(mock)
	client := &thriftclient.MonitoredClient{Client: recorder}
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

func TestMonitoredClient(t *testing.T) {
	cases := []struct {
		name          string
		call          thriftclient.MockCall
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
				span := tracing.AsSpan(s)
				if span.Name() != method {
					t.Errorf("span name mismatch, expected %q, got %q", method, span.Name())
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
