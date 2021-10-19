package thriftbp_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/reddit/baseplate.go/ecinterface"
	"github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
	"github.com/reddit/baseplate.go/mqsend"
	"github.com/reddit/baseplate.go/thriftbp"
	"github.com/reddit/baseplate.go/thriftbp/thrifttest"
	"github.com/reddit/baseplate.go/tracing"
	"github.com/reddit/baseplate.go/transport"
)

const (
	testTimeout = time.Millisecond * 100
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

func TestInjectServerSpan(t *testing.T) {
	const ua = "foo"

	defer func() {
		tracing.CloseTracer()
		tracing.InitGlobalTracer(tracing.Config{})
	}()
	mmq := mqsend.OpenMockMessageQueue(mqsend.MessageQueueConfig{
		MaxQueueSize:   100,
		MaxMessageSize: 1024,
	})
	logger, startFailing := tracing.TestWrapper(t)
	tracing.InitGlobalTracer(tracing.Config{
		SampleRate:               1,
		MaxRecordTimeout:         testTimeout,
		Logger:                   logger,
		TestOnlyMockMessageQueue: mmq,
	})
	startFailing()
	name := "test"
	processor := thrifttest.NewMockTProcessor(
		t,
		map[string]thrift.TProcessorFunction{
			name: thrift.WrappedTProcessorFunction{
				Wrapped: func(ctx context.Context, seqID int32, in, out thrift.TProtocol) (bool, thrift.TException) {
					return false, thrift.WrapTException(errors.New("TError"))
				},
			},
		},
	)
	ctx := context.Background()
	ctx = thrift.SetHeader(ctx, transport.HeaderTracingSampled, transport.HeaderTracingSampledTrue)
	ctx = thrift.SetHeader(ctx, transport.HeaderUserAgent, ua)
	ctx = thrifttest.SetMockTProcessorName(ctx, name)

	wrapped := thrift.WrapProcessor(processor, thriftbp.InjectServerSpan(nil))
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
	var hasError, hasPeerService bool
	for _, annotation := range trace.BinaryAnnotations {
		if annotation.Key == "error" {
			hasError = true
		}
		if annotation.Key == tracing.TagKeyPeerService {
			hasPeerService = true
			v, ok := annotation.Value.(string)
			if !ok || v != ua {
				t.Errorf(
					"Expected binary annotation of %q to be %q, got %#v, %q, %v",
					tracing.TagKeyPeerService,
					ua,
					annotation.Value,
					v,
					ok,
				)
			}
		}
	}
	if !hasError {
		t.Error("Error binary annotation was not present.")
	}
	if !hasPeerService {
		t.Errorf("%q binary annotation was not present.", tracing.TagKeyPeerService)
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

func TestPrometheusMetricsMiddleware(t *testing.T) {
	t.Run("bp error", func(t *testing.T) {
		te := thrift.NewTProtocolExceptionWithType(thrift.PROTOCOL_ERROR, thriftbp.WrapBaseplateError(errors.New("test")))
		next := thrift.WrappedTProcessorFunction{
			Wrapped: func(ctx context.Context, seqId int32, in, out thrift.TProtocol) (bool, thrift.TException) {
				return false, te
			},
		}
		promMiddleware := thriftbp.PrometheusMetrics("service")
		wrapped := promMiddleware("testmethod", next)
		ok, err := wrapped.Process(context.Background(), 1, nil, nil)
		if ok {
			t.Errorf("expected ok to be false, got true")
		}
		if err == nil {
			t.Error("expected an error, got nil")
		}
	})

	t.Run("application error", func(t *testing.T) {
		te := thrift.NewTApplicationException(thrift.UNKNOWN_METHOD, "unknown err mdsg")
		next := thrift.WrappedTProcessorFunction{
			Wrapped: func(ctx context.Context, seqId int32, in, out thrift.TProtocol) (bool, thrift.TException) {
				return false, te
			},
		}
		promMiddleware := thriftbp.PrometheusMetrics("service")
		wrapped := promMiddleware("testmethod", next)
		ok, err := wrapped.Process(context.Background(), 1, nil, nil)
		if ok {
			t.Errorf("expected ok to be false, got true")
		}
		if err == nil {
			t.Error("expected an error, got nil")
		}
	})

	t.Run("compile error", func(t *testing.T) {
		te := baseplate.NewError()
		next := thrift.WrappedTProcessorFunction{
			Wrapped: func(ctx context.Context, seqId int32, in, out thrift.TProtocol) (bool, thrift.TException) {
				return false, te
			},
		}
		promMiddleware := thriftbp.PrometheusMetrics("service")
		wrapped := promMiddleware("testmethod", next)
		ok, err := wrapped.Process(context.Background(), 1, nil, nil)
		if ok {
			t.Errorf("expected ok to be false, got true")
		}
		if err == nil {
			t.Error("expected an error, got nil")
		}
	})

	t.Run("success", func(t *testing.T) {
		next := thrift.WrappedTProcessorFunction{
			Wrapped: func(ctx context.Context, seqId int32, in, out thrift.TProtocol) (bool, thrift.TException) {
				return true, nil
			},
		}
		promMiddleware := thriftbp.PrometheusMetrics("service")
		wrapped := promMiddleware("testmethod", next)
		ok, err := wrapped.Process(context.Background(), 1, nil, nil)
		if !ok {
			t.Errorf("expected ok to be true, got false")
		}
		if err != nil {
			t.Error("expected nil, got an error")
		}
	})
}
