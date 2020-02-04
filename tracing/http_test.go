package tracing

import (
	"context"
	"testing"

	"github.com/reddit/baseplate.go/httpbp"
	"github.com/reddit/baseplate.go/log"
)

func TestStartSpanFromHTTPContext(t *testing.T) {
	const (
		name = "foo"

		traceInt = 12345
		traceStr = "12345"

		spanInt = 54321
		spanStr = "54321"
	)

	tracer := Tracer{
		Logger: log.TestWrapper(t),
	}

	ctx := context.Background()
	ctx = httpbp.SetHeader(ctx, httpbp.TraceIDContextKey, traceStr)
	ctx = httpbp.SetHeader(ctx, httpbp.SpanIDContextKey, spanStr)

	ctx, span := StartSpanFromHTTPContextWithTracer(ctx, name, &tracer)
	zs := span.trace.toZipkinSpan()

	if zs.TraceID != traceInt {
		t.Errorf(
			"span's traceID expected %d, got %d",
			traceInt,
			zs.TraceID,
		)
	}

	if zs.ParentID != spanInt {
		t.Errorf(
			"span's parent id expected %d, got %d",
			spanInt,
			zs.ParentID,
		)
	}
}
