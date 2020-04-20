package thriftbp_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/reddit/baseplate.go/edgecontext"
	"github.com/reddit/baseplate.go/mqsend"
	"github.com/reddit/baseplate.go/thriftbp"
	"github.com/reddit/baseplate.go/tracing"
)

const (
	testTimeout = time.Millisecond * 100
)

func testProcessorMiddleware(c *counter) thriftbp.ProcessorMiddleware {
	return func(name string, next thrift.TProcessorFunction) thrift.TProcessorFunction {
		return thriftbp.WrappedTProcessorFunc{
			Wrapped: func(ctx context.Context, seqId int32, in, out thrift.TProtocol) (bool, thrift.TException) {
				c.incr()
				return next.Process(ctx, seqId, in, out)
			},
		}
	}
}

type edgecontextRecorder struct {
	EdgeContext *edgecontext.EdgeRequestContext
}

func edgecontextRecorderMiddleware(recorder *edgecontextRecorder) thriftbp.ProcessorMiddleware {
	return func(name string, next thrift.TProcessorFunction) thrift.TProcessorFunction {
		return thriftbp.WrappedTProcessorFunc{
			Wrapped: func(ctx context.Context, seqId int32, in, out thrift.TProtocol) (bool, thrift.TException) {
				recorder.EdgeContext, _ = edgecontext.GetEdgeContext(ctx)
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
	wrapped := thriftbp.WrapProcessor(processor, testProcessorMiddleware(c))
	wrapped.Process(ctx, nil, nil)
	if c.count != 1 {
		t.Fatalf("Unexpected count value %v", c.count)
	}
}

func TestInjectServerSpan(t *testing.T) {
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

	wrapped := thriftbp.WrapProcessor(processor, thriftbp.InjectServerSpan)
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

func TestInitializeEdgeContext(t *testing.T) {
	store, dir := newSecretsStore(t)
	defer os.RemoveAll(dir)
	defer store.Close()

	const expectedID = "t2_example"
	impl := edgecontext.Init(edgecontext.Config{Store: store})

	ctx := thrift.SetHeader(
		context.Background(),
		thriftbp.HeaderEdgeRequest,
		headerWithValidAuth,
	)

	ctx = thriftbp.InitializeEdgeContext(ctx, impl)
	ec, ok := edgecontext.GetEdgeContext(ctx)
	if !ok {
		t.Error("EdgeRequestContext not set on context")
	}

	userID, ok := ec.User().ID()
	if !ok {
		t.Error("user should be logged in")
	}
	if userID != expectedID {
		t.Errorf("user ID mismatch, expected %q, got %q", expectedID, userID)
	}
}

func TestInjectEdgeContext(t *testing.T) {
	const expectedID = "t2_example"

	store, dir := newSecretsStore(t)
	defer os.RemoveAll(dir)
	defer store.Close()

	impl := edgecontext.Init(edgecontext.Config{Store: store})

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

	ctx := thrift.SetHeader(
		context.Background(),
		thriftbp.HeaderEdgeRequest,
		headerWithValidAuth,
	)
	ctx = thriftbp.SetMockBaseplateProcessorName(ctx, name)
	recorder := edgecontextRecorder{}
	wrapped := thriftbp.WrapProcessor(
		processor,
		thriftbp.InjectEdgeContext(impl),
		edgecontextRecorderMiddleware(&recorder),
	)
	wrapped.Process(ctx, nil, nil)
	if recorder.EdgeContext == nil {
		t.Fatal("edge context not set")
	}

	userID, ok := recorder.EdgeContext.User().ID()
	if !ok {
		t.Fatal("user should be logged in")
	}
	if userID != expectedID {
		t.Fatalf("user ID does not match, expected %q, got %q", expectedID, userID)
	}
}
