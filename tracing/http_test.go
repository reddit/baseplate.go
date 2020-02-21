package tracing

import (
	"context"
	"testing"

	"github.com/reddit/baseplate.go/httpbp"
)

func TestStartSpanFromHTTPContext(t *testing.T) {
	const (
		name = "foo"

		traceInt = 12345
		traceStr = "12345"

		spanInt = 54321
		spanStr = "54321"
	)

	defer func() {
		CloseTracer()
		InitGlobalTracer(TracerConfig{})
	}()
	logger, startFailing := TestWrapper(t)
	InitGlobalTracer(TracerConfig{
		Logger: logger,
	})
	startFailing()

	ctx := context.Background()
	ctx = httpbp.SetHeader(ctx, httpbp.TraceIDContextKey, traceStr)
	ctx = httpbp.SetHeader(ctx, httpbp.SpanIDContextKey, spanStr)

	ctx, span := StartSpanFromHTTPContext(ctx, name)
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
