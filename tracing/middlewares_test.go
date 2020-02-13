package tracing_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/reddit/baseplate.go/thriftbp"

	"github.com/reddit/baseplate.go/httpbp"
	"github.com/reddit/baseplate.go/mqsend"
	"github.com/reddit/baseplate.go/runtimebp"
	"github.com/reddit/baseplate.go/tracing"
)

func testEndpoint(ctx context.Context, request interface{}) (interface{}, error) {
	return nil, errors.New("test error")
}

func TestInjectHTTPServerSpanWithTracer(t *testing.T) {
	ip, err := runtimebp.GetFirstIPv4()
	if err != nil {
		t.Logf("Unable to get local ip address: %v", err)
	}
	tracer := tracing.Tracer{
		SampleRate: 1,
		Endpoint: tracing.ZipkinEndpointInfo{
			ServiceName: "test-service",
			IPv4:        ip,
		},
	}

	tracer.Recorder = mqsend.OpenMockMessageQueue(mqsend.MessageQueueConfig{
		MaxQueueSize:   100,
		MaxMessageSize: 1024,
	})

	ctx := context.Background()
	ctx = httpbp.SetHeader(ctx, httpbp.SpanSampledContextKey, "1")
	middleware := tracing.InjectHTTPServerSpanWithTracer("test", &tracer)
	wrapped := middleware(testEndpoint)
	wrapped(ctx, nil)
	mmq := tracer.Recorder.(*mqsend.MockMessageQueue)
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	msg, err := mmq.Receive(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Encoded span: %s", msg)

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
	if !hasError {
		t.Error("Error binary annotation was not present.")
	}
}

func TestInjectThriftServerSpanWithTracer(t *testing.T) {
	ip, err := runtimebp.GetFirstIPv4()
	if err != nil {
		t.Logf("Unable to get local ip address: %v", err)
	}
	tracer := tracing.Tracer{
		SampleRate: 1,
		Endpoint: tracing.ZipkinEndpointInfo{
			ServiceName: "test-service",
			IPv4:        ip,
		},
	}

	tracer.Recorder = mqsend.OpenMockMessageQueue(mqsend.MessageQueueConfig{
		MaxQueueSize:   100,
		MaxMessageSize: 1024,
	})

	ctx := context.Background()
	ctx = thrift.SetHeader(ctx, thriftbp.HeaderTracingSampled, thriftbp.HeaderTracingSampledTrue)
	middleware := tracing.InjectThriftServerSpanWithTracer("test", &tracer)
	wrapped := middleware(testEndpoint)
	wrapped(ctx, nil)
	mmq := tracer.Recorder.(*mqsend.MockMessageQueue)
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	msg, err := mmq.Receive(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Encoded span: %s", msg)

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
	if !hasError {
		t.Error("Error binary annotation was not present.")
	}
}
