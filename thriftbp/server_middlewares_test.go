package thriftbp_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/reddit/baseplate.go/ecinterface"
	"github.com/reddit/baseplate.go/thriftbp"
	"github.com/reddit/baseplate.go/thriftbp/thrifttest"
	"github.com/reddit/baseplate.go/tracing"
	"github.com/reddit/baseplate.go/transport"
)

type edgecontextRecorder struct {
	header string
}

func edgecontextRecorderMiddleware(impl ecinterface.Interface, recorder *edgecontextRecorder) thrift.ProcessorMiddleware {
	return func(name string, next thrift.TProcessorFunction) thrift.TProcessorFunction {
		return thrift.WrappedTProcessorFunction{
			Wrapped: func(ctx context.Context, seqID int32, in, out thrift.TProtocol) (bool, thrift.TException) {
				recorder.header, _ = impl.ContextToHeader(ctx)
				return next.Process(ctx, seqID, in, out)
			},
		}
	}
}

func TestStartSpanFromThriftContext(t *testing.T) {
	const (
		name   = "foo"
		trace  = "12345"
		spanID = "54321"
	)

	defer func() {
		tracing.CloseTracer()
		tracing.InitGlobalTracer(tracing.Config{})
	}()
	logger, startFailing := tracing.TestWrapper(t)
	tracing.InitGlobalTracer(tracing.Config{
		Logger: logger,
	})
	startFailing()

	ctx := context.Background()
	ctx = thrift.SetHeader(ctx, transport.HeaderTracingTrace, trace)
	ctx = thrift.SetHeader(ctx, transport.HeaderTracingSpan, spanID)

	_, span := thriftbp.StartSpanFromThriftContext(ctx, name)

	if span.TraceID() != trace {
		t.Errorf(
			"span's traceID expected %q, got %q",
			trace,
			span.TraceID(),
		)
	}

	if span.ParentID() != spanID {
		t.Errorf(
			"span's parent id expected %q, got %q",
			spanID,
			span.ParentID(),
		)
	}
}

func TestInitializeEdgeContext(t *testing.T) {
	const expectedHeader = "dummy-edge-context"

	impl := ecinterface.Mock()

	ctx := thrift.SetHeader(
		context.Background(),
		transport.HeaderEdgeRequest,
		expectedHeader,
	)

	ctx = thriftbp.InitializeEdgeContext(ctx, impl)
	header, ok := impl.ContextToHeader(ctx)
	if !ok {
		t.Error("EdgeRequestContext not set on context")
	}
	if header != expectedHeader {
		t.Errorf("Header expected %q, got %q", expectedHeader, header)
	}
}

func TestInjectEdgeContext(t *testing.T) {
	const expectedHeader = "dummy-edge-context"

	impl := ecinterface.Mock()

	name := "test"
	processor := thrifttest.NewMockTProcessor(
		t,
		map[string]thrift.TProcessorFunction{
			name: thrift.WrappedTProcessorFunction{
				Wrapped: func(ctx context.Context, seqID int32, in, out thrift.TProtocol) (bool, thrift.TException) {
					return true, nil
				},
			},
		},
	)

	ctx := thrift.SetHeader(
		context.Background(),
		transport.HeaderEdgeRequest,
		expectedHeader,
	)
	ctx = thrifttest.SetMockTProcessorName(ctx, name)
	recorder := edgecontextRecorder{}
	wrapped := thrift.WrapProcessor(
		processor,
		thriftbp.InjectEdgeContext(impl),
		edgecontextRecorderMiddleware(impl, &recorder),
	)
	wrapped.Process(ctx, nil, nil)
	if recorder.header != expectedHeader {
		t.Errorf("Expected edge-context header %q, got %q", expectedHeader, recorder.header)
	}
}

func TestExtractDeadlineBudget(t *testing.T) {
	name := "test"
	processor := func(checker func(context.Context)) thrift.TProcessor {
		return thrifttest.NewMockTProcessor(
			t,
			map[string]thrift.TProcessorFunction{
				name: thrift.WrappedTProcessorFunction{
					Wrapped: func(ctx context.Context, seqID int32, in, out thrift.TProtocol) (bool, thrift.TException) {
						checker(ctx)
						return true, nil
					},
				},
			},
		)
	}

	t.Run(
		"invalid",
		func(t *testing.T) {
			ctx := thrift.SetHeader(
				context.Background(),
				transport.HeaderDeadlineBudget,
				"foobar",
			)
			ctx = thrifttest.SetMockTProcessorName(ctx, name)
			wrapped := thrift.WrapProcessor(
				processor(func(ctx context.Context) {
					deadline, ok := ctx.Deadline()
					if ok {
						t.Errorf("Expected no deadline set, got %v", time.Until(deadline))
					}
				}),
				thriftbp.ExtractDeadlineBudget,
			)
			wrapped.Process(ctx, nil, nil)
		},
	)

	t.Run(
		"<1",
		func(t *testing.T) {
			ctx := thrift.SetHeader(
				context.Background(),
				transport.HeaderDeadlineBudget,
				"0",
			)
			ctx = thrifttest.SetMockTProcessorName(ctx, name)
			wrapped := thrift.WrapProcessor(
				processor(func(ctx context.Context) {
					deadline, ok := ctx.Deadline()
					if ok {
						t.Errorf("Expected no deadline set, got %v", time.Until(deadline))
					}
				}),
				thriftbp.ExtractDeadlineBudget,
			)
			wrapped.Process(ctx, nil, nil)
		},
	)

	t.Run(
		"50",
		func(t *testing.T) {
			ctx := thrift.SetHeader(
				context.Background(),
				transport.HeaderDeadlineBudget,
				"50",
			)
			ctx = thrifttest.SetMockTProcessorName(ctx, name)
			wrapped := thrift.WrapProcessor(
				processor(func(ctx context.Context) {
					deadline, ok := ctx.Deadline()
					if !ok {
						t.Fatal("Deadline not set")
					}

					duration := time.Until(deadline)
					if duration.Round(time.Millisecond).Milliseconds() != 50 {
						t.Errorf("Expected deadline to be 50ms, got %v", duration)
					}
				}),
				thriftbp.ExtractDeadlineBudget,
			)
			wrapped.Process(ctx, nil, nil)
		},
	)
}

func TestPanicMiddleware(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		panicErr := errors.New("oops")
		next := thrift.WrappedTProcessorFunction{
			Wrapped: func(ctx context.Context, seqId int32, in, out thrift.TProtocol) (bool, thrift.TException) {
				panic(panicErr)
			},
		}
		wrapped := thriftbp.RecoverPanic("test", next)
		ok, err := wrapped.Process(context.Background(), 1, nil, nil)
		if ok {
			t.Errorf("expected ok to be false, got true")
		}
		if !errors.Is(err, panicErr) {
			t.Errorf("error mismatch, expectd %v, got %v", panicErr, err)
		}
	})

	t.Run("non-error", func(t *testing.T) {
		next := thrift.WrappedTProcessorFunction{
			Wrapped: func(ctx context.Context, seqId int32, in, out thrift.TProtocol) (bool, thrift.TException) {
				panic("oops")
			},
		}
		wrapped := thriftbp.RecoverPanic("test", next)
		ok, err := wrapped.Process(context.Background(), 1, nil, nil)
		if ok {
			t.Errorf("expected ok to be false, got true")
		}
		if err == nil {
			t.Error("expected an error, got nil")
		}
	})
}
