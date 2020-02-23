package tracing_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/reddit/baseplate.go/httpbp"
	"github.com/reddit/baseplate.go/mqsend"
	"github.com/reddit/baseplate.go/thriftbp"
	"github.com/reddit/baseplate.go/tracing"
)

const testTimeout = time.Millisecond * 100

func testEndpoint(ctx context.Context, request interface{}) (interface{}, error) {
	return nil, errors.New("test error")
}

func TestInjectHTTPServerSpanWithTracer(t *testing.T) {
	defer func() {
		tracing.CloseTracer()
		tracing.InitGlobalTracer(tracing.TracerConfig{})
	}()
	mmq := mqsend.OpenMockMessageQueue(mqsend.MessageQueueConfig{
		MaxQueueSize:   100,
		MaxMessageSize: 1024,
	})
	logger, startFailing := tracing.TestWrapper(t)
	tracing.InitGlobalTracer(tracing.TracerConfig{
		SampleRate:               1,
		MaxRecordTimeout:         testTimeout,
		Logger:                   logger,
		TestOnlyMockMessageQueue: mmq,
	})
	startFailing()

	ctx := context.Background()
	ctx = httpbp.SetHeader(ctx, httpbp.SpanSampledContextKey, "1")
	middleware := tracing.InjectHTTPServerSpan("test")
	wrapped := middleware(testEndpoint)
	wrapped(ctx, nil)
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
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
	defer func() {
		tracing.CloseTracer()
		tracing.InitGlobalTracer(tracing.TracerConfig{})
	}()
	mmq := mqsend.OpenMockMessageQueue(mqsend.MessageQueueConfig{
		MaxQueueSize:   100,
		MaxMessageSize: 1024,
	})
	logger, startFailing := tracing.TestWrapper(t)
	tracing.InitGlobalTracer(tracing.TracerConfig{
		SampleRate:               1,
		MaxRecordTimeout:         testTimeout,
		Logger:                   logger,
		TestOnlyMockMessageQueue: mmq,
	})
	startFailing()

	ctx := context.Background()
	ctx = thrift.SetHeader(ctx, thriftbp.HeaderTracingSampled, thriftbp.HeaderTracingSampledTrue)
	middleware := tracing.InjectThriftServerSpan("test")
	wrapped := middleware(testEndpoint)
	wrapped(ctx, nil)
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
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
