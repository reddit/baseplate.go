package thriftbp_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/apache/thrift/lib/go/thrift"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/reddit/baseplate.go/thriftbp"
	"github.com/reddit/baseplate.go/tracing"
)

func TestCreateThriftContextFromSpan(t *testing.T) {
	const (
		name    = "foo"
		traceID = "12345"
		spanID  = "54321"
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

	checkContextKey := func(t *testing.T, ctx context.Context, key string) {
		t.Helper()

		if _, ok := thrift.GetHeader(ctx, key); !ok {
			t.Errorf("context should have %s", key)
		}

		headers := thrift.GetWriteHeaderList(ctx)
		var found bool
		for _, header := range headers {
			if header == key {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("write header list should have %s", key)
		}
	}

	parentCtx := context.Background()
	parentCtx = thrift.SetHeader(parentCtx, thriftbp.HeaderTracingTrace, traceID)
	parentCtx = thrift.SetHeader(parentCtx, thriftbp.HeaderTracingSpan, spanID)

	_, span := thriftbp.StartSpanFromThriftContext(parentCtx, name)

	t.Run(
		"not-sampled-and-new",
		func(t *testing.T) {
			ctx := context.Background()
			child := tracing.AsSpan(opentracing.StartSpan(
				"test",
				opentracing.ChildOf(span),
				tracing.SpanTypeOption{Type: tracing.SpanTypeClient},
			))
			ctx = thriftbp.CreateThriftContextFromSpan(ctx, child)
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
			expectedParentID := fmt.Sprintf("%v", child.ParentID())
			if v, ok := thrift.GetHeader(ctx, thriftbp.HeaderTracingParent); !ok || v != expectedParentID {
				t.Errorf(
					"parent in the context expected to be %q, got %q & %v",
					expectedParentID,
					v,
					ok,
				)
			}

			checkContextKey(t, ctx, thriftbp.HeaderTracingSpan)
			expectedSpanID := fmt.Sprintf("%d", child.ID())
			if v, ok := thrift.GetHeader(ctx, thriftbp.HeaderTracingSpan); !ok || v != expectedSpanID {
				t.Errorf(
					"span in the context expected to be %q, got %q & %v",
					expectedSpanID,
					v,
					ok,
				)
			}

			if v, ok := thrift.GetHeader(ctx, thriftbp.HeaderTracingSampled); ok {
				t.Errorf(
					"sampled should not be in the context, got %q & %v",
					v,
					ok,
				)
			}
		},
	)

	parentCtx = thrift.SetHeader(parentCtx, thriftbp.HeaderTracingSampled, thriftbp.HeaderTracingSampledTrue)
	_, span = thriftbp.StartSpanFromThriftContext(parentCtx, name)

	t.Run(
		"sampled-and-overwrite",
		func(t *testing.T) {
			ctx := context.Background()
			ctx = thrift.SetWriteHeaderList(ctx, thriftbp.HeadersToForward)
			child := tracing.AsSpan(opentracing.StartSpan(
				"test",
				opentracing.ChildOf(span),
				tracing.SpanTypeOption{Type: tracing.SpanTypeClient},
			))
			ctx = thriftbp.CreateThriftContextFromSpan(ctx, child)

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
			expectedParentID := fmt.Sprintf("%v", child.ParentID())
			if v, ok := thrift.GetHeader(ctx, thriftbp.HeaderTracingParent); !ok || v != expectedParentID {
				t.Errorf(
					"parent in the context expected to be %q, got %q & %v",
					expectedParentID,
					v,
					ok,
				)
			}

			checkContextKey(t, ctx, thriftbp.HeaderTracingSpan)
			expectedSpanID := fmt.Sprintf("%d", child.ID())
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
