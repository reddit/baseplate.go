package tracing_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"testing"

	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/set"
	"github.com/reddit/baseplate.go/thriftbp"
	"github.com/reddit/baseplate.go/tracing"

	"github.com/apache/thrift/lib/go/thrift"
)

func TestMain(m *testing.M) {
	var ret int
	defer func() {
		os.Exit(ret)
	}()

	listener, err := net.Listen("tcp", "localhost:")
	if err != nil {
		panic(err)
	}
	var wg sync.WaitGroup
	defer wg.Wait()
	var httpServer http.Server
	wg.Add(1)
	go func() {
		defer wg.Done()
		httpServer.Serve(listener)
	}()
	defer httpServer.Close()
	if err := tracing.InitTracer("my service", listener.Addr().String(), 1); err != nil {
		panic(err)
	}
	defer tracing.CloseZipkinReporter()

	ret = m.Run()
}

func TestStartSpanFromThriftContext(t *testing.T) {
	const (
		optName = "foo"

		traceInt = 12345
		traceStr = "12345"

		spanInt = 54321
		spanStr = "54321"
	)

	ctx := context.Background()
	ctx = thrift.SetHeader(ctx, thriftbp.HeaderTracingTrace, traceStr)
	ctx = thrift.SetHeader(ctx, thriftbp.HeaderTracingSpan, spanStr)

	t.Run(
		"not-sampled",
		func(t *testing.T) {
			span := tracing.StartSpanFromThriftContext(ctx, optName, log.TestWrapper(t))
			zipkinCtx := span.Context()
			t.Logf("span: %+v, context: %+v", span, zipkinCtx)

			if zipkinCtx.TraceID.Low != traceInt {
				t.Errorf(
					"span context's low trace id expected %d, got %d",
					traceInt,
					zipkinCtx.TraceID.Low,
				)
			}

			if zipkinCtx.TraceID.High != 0 {
				t.Errorf(
					"span context's low trace id expected 0, got %d",
					zipkinCtx.TraceID.High,
				)
			}

			if zipkinCtx.ParentID == nil {
				t.Error("span context's parent id shouldn't be nil")
			} else if *zipkinCtx.ParentID != spanInt {
				t.Errorf(
					"span context's parent id expected %d, got %d",
					spanInt,
					*zipkinCtx.ParentID,
				)
			}

			if zipkinCtx.Sampled == nil {
				t.Error("span context's sampled shouldn't be nil")
			} else if *zipkinCtx.Sampled {
				t.Error("span context's sampled should be false")
			}
		},
	)

	ctx = thrift.SetHeader(ctx, thriftbp.HeaderTracingSampled, thriftbp.HeaderTracingSampledTrue)

	t.Run(
		"sampled",
		func(t *testing.T) {
			span := tracing.StartSpanFromThriftContext(ctx, optName, log.TestWrapper(t))
			zipkinCtx := span.Context()
			t.Logf("span: %+v, context: %+v", span, zipkinCtx)

			if zipkinCtx.TraceID.Low != traceInt {
				t.Errorf(
					"span context's low trace id expected %d, got %d",
					traceInt,
					zipkinCtx.TraceID.Low,
				)
			}

			if zipkinCtx.TraceID.High != 0 {
				t.Errorf(
					"span context's low trace id expected 0, got %d",
					zipkinCtx.TraceID.High,
				)
			}

			if zipkinCtx.ParentID == nil {
				t.Error("span context's parent id shouldn't be nil")
			} else if *zipkinCtx.ParentID != spanInt {
				t.Errorf(
					"span context's parent id expected %d, got %d",
					spanInt,
					*zipkinCtx.ParentID,
				)
			}

			if zipkinCtx.Sampled == nil {
				t.Error("span context's sampled shouldn't be nil")
			} else if !*zipkinCtx.Sampled {
				t.Error("span context's sampled should be true")
			}
		},
	)
}

func TestCreateThriftContextFromSpan(t *testing.T) {
	const (
		optName = "foo"
		traceID = "12345"
		spanID  = "54321"
	)

	checkContextKey := func(t *testing.T, ctx context.Context, key string) {
		t.Helper()

		headersSet := set.StringSliceToSet(thrift.GetWriteHeaderList(ctx))
		if !headersSet.Contains(key) {
			t.Errorf("context should have %s", key)
		}
	}

	parentCtx := context.Background()
	parentCtx = thrift.SetHeader(parentCtx, thriftbp.HeaderTracingTrace, traceID)
	parentCtx = thrift.SetHeader(parentCtx, thriftbp.HeaderTracingSpan, spanID)
	span := tracing.StartSpanFromThriftContext(parentCtx, optName, log.TestWrapper(t))

	t.Run(
		"not-sampled-and-new",
		func(t *testing.T) {
			ctx := context.Background()
			ctx = tracing.CreateThriftContextFromSpan(ctx, span)
			checkContextKey(t, ctx, thriftbp.HeaderTracingTrace)
			if v, ok := thrift.GetHeader(ctx, thriftbp.HeaderTracingTrace); !ok || v != traceID {
				t.Errorf(
					"trace in the context expected to be %q, got %q & %v",
					traceID,
					v,
					ok,
				)
			}

			checkContextKey(t, ctx, thriftbp.HeaderTracingParent)
			if v, ok := thrift.GetHeader(ctx, thriftbp.HeaderTracingParent); !ok || v != spanID {
				t.Errorf(
					"parent in the context expected to be %q, got %q & %v",
					spanID,
					v,
					ok,
				)
			}

			checkContextKey(t, ctx, thriftbp.HeaderTracingSpan)
			expectedSpanID := fmt.Sprintf("%d", span.Context().ID)
			if v, ok := thrift.GetHeader(ctx, thriftbp.HeaderTracingSpan); !ok || v != expectedSpanID {
				t.Errorf(
					"span in the context expected to be %q, got %q & %v",
					expectedSpanID,
					v,
					ok,
				)
			}

			checkContextKey(t, ctx, thriftbp.HeaderTracingSampled)
			if v, ok := thrift.GetHeader(ctx, thriftbp.HeaderTracingSampled); !ok || v == thriftbp.HeaderTracingSampledTrue {
				t.Errorf(
					"sampled in the context expected to be not %q, got %q & %v",
					thriftbp.HeaderTracingSampledTrue,
					v,
					ok,
				)
			}
		},
	)

	parentCtx = thrift.SetHeader(parentCtx, thriftbp.HeaderTracingSampled, thriftbp.HeaderTracingSampledTrue)
	span = tracing.StartSpanFromThriftContext(parentCtx, optName, log.TestWrapper(t))

	t.Run(
		"sampled-and-overwrite",
		func(t *testing.T) {
			ctx := context.Background()
			ctx = thrift.SetWriteHeaderList(ctx, thriftbp.HeadersToForward)
			ctx = tracing.CreateThriftContextFromSpan(ctx, span)

			headers := thrift.GetWriteHeaderList(ctx)
			headersSet := set.StringSliceToSet(headers)
			if len(headers) != len(headersSet) {
				t.Errorf(
					"Expected no duplications in write header list, got %+v",
					headers,
				)
			}

			checkContextKey(t, ctx, thriftbp.HeaderTracingTrace)
			if v, ok := thrift.GetHeader(ctx, thriftbp.HeaderTracingTrace); !ok || v != traceID {
				t.Errorf(
					"trace in the context expected to be %q, got %q & %v",
					traceID,
					v,
					ok,
				)
			}

			checkContextKey(t, ctx, thriftbp.HeaderTracingParent)
			if v, ok := thrift.GetHeader(ctx, thriftbp.HeaderTracingParent); !ok || v != spanID {
				t.Errorf(
					"parent in the context expected to be %q, got %q & %v",
					spanID,
					v,
					ok,
				)
			}

			checkContextKey(t, ctx, thriftbp.HeaderTracingSpan)
			expectedSpanID := fmt.Sprintf("%d", span.Context().ID)
			if v, ok := thrift.GetHeader(ctx, thriftbp.HeaderTracingSpan); !ok || v != expectedSpanID {
				t.Errorf(
					"span in the context expected to be %q, got %q & %v",
					expectedSpanID,
					v,
					ok,
				)
			}

			checkContextKey(t, ctx, thriftbp.HeaderTracingSampled)
			if v, ok := thrift.GetHeader(ctx, thriftbp.HeaderTracingSampled); !ok || v != thriftbp.HeaderTracingSampledTrue {
				t.Errorf(
					"sampled in the context expected to be %q, got %q & %v",
					thriftbp.HeaderTracingSampledTrue,
					v,
					ok,
				)
			}
		},
	)
}
