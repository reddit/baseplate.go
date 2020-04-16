package thriftbp_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/reddit/baseplate.go/mqsend"
	"github.com/reddit/baseplate.go/thriftbp"
	"github.com/reddit/baseplate.go/tracing"
)

const (
	testTimeout = time.Millisecond * 100
)

type counter struct {
	count int
}

func (c *counter) Incr() {
	c.count++
}

func testMiddleware(c *counter) thriftbp.Middleware {
	return func(name string, next thrift.TProcessorFunction) thrift.TProcessorFunction {
		return thriftbp.WrappedTProcessorFunc{
			Wrapped: func(ctx context.Context, seqId int32, in, out thrift.TProtocol) (bool, thrift.TException) {
				c.Incr()
				return next.Process(ctx, seqId, in, out)
			},
		}
	}
}

func TestWrap(t *testing.T) {
	name := "test"
	processor := thriftbp.NewMockBaseplateProcessor(
		map[string]thrift.TProcessorFunction{
			name: thriftbp.WrappedTProcessorFunc{
				Wrapped: func(ctx context.Context, seqId int32, in, out thrift.TProtocol) (bool, thrift.TException) {
					return true, nil
				},
			},
		},
	)
	c := &counter{}
	if c.count != 0 {
		t.Fatal("Unexpected initial count.")
	}
	ctx := context.Background()
	ctx = thrift.SetHeader(ctx, thriftbp.HeaderTracingSampled, thriftbp.HeaderTracingSampledTrue)
	ctx = thriftbp.SetMockBaseplateProcessorName(ctx, name)
	wrapped := thriftbp.Wrap(processor, testMiddleware(c))
	wrapped.Process(ctx, nil, nil)
	if c.count != 1 {
		t.Fatalf("Unexpected count value %v", c.count)
	}
}

func TestInjectThriftServerSpan(t *testing.T) {
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
	name := "test"
	processor := thriftbp.NewMockBaseplateProcessor(
		map[string]thrift.TProcessorFunction{
			name: thriftbp.WrappedTProcessorFunc{
				Wrapped: func(ctx context.Context, seqId int32, in, out thrift.TProtocol) (bool, thrift.TException) {
					return false, errors.New("TError")
				},
			},
		},
	)
	ctx := context.Background()
	ctx = thrift.SetHeader(ctx, thriftbp.HeaderTracingSampled, thriftbp.HeaderTracingSampledTrue)
	ctx = thriftbp.SetMockBaseplateProcessorName(ctx, name)

	wrapped := thriftbp.Wrap(processor, thriftbp.InjectThriftServerSpan)
	wrapped.Process(ctx, nil, nil)
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

func TestStartSpanFromThriftContext(t *testing.T) {
	const (
		name = "foo"

		traceInt = 12345
		traceStr = "12345"

		spanInt = 54321
		spanStr = "54321"
	)

	defer func() {
		tracing.CloseTracer()
		tracing.InitGlobalTracer(tracing.TracerConfig{})
	}()
	logger, startFailing := tracing.TestWrapper(t)
	tracing.InitGlobalTracer(tracing.TracerConfig{
		Logger: logger,
	})
	startFailing()

	ctx := context.Background()
	ctx = thrift.SetHeader(ctx, thriftbp.HeaderTracingTrace, traceStr)
	ctx = thrift.SetHeader(ctx, thriftbp.HeaderTracingSpan, spanStr)

	ctx, span := thriftbp.StartSpanFromThriftContext(ctx, name)

	if span.TraceID() != traceInt {
		t.Errorf(
			"span's traceID expected %d, got %d",
			traceInt,
			span.TraceID(),
		)
	}

	if span.ParentID() != spanInt {
		t.Errorf(
			"span's parent id expected %d, got %d",
			spanInt,
			span.ParentID(),
		)
	}
}
