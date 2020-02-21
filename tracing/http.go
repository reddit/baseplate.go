package tracing

import (
	"context"
	"fmt"
	"strconv"

	opentracing "github.com/opentracing/opentracing-go"

	"github.com/reddit/baseplate.go/httpbp"
	"github.com/reddit/baseplate.go/thriftbp"
)

// StartSpanFromHTTPContext creates a server span from http context object.
//
// This span would usually be used as the span of the whole http endpoint
// handler, and the parent of the child-spans.
//
// Please note that "Sampled" header is default to false according to baseplate
// spec, so if the context object doesn't have headers injected correctly,
// this span (and all its child-spans) will never be sampled,
// unless debug flag was set explicitly later.
//
// Caller should pass in the context object they got from go-kit http library
// with httpbp.PopulateRequestContext as a ServerBefore hook, this way the
// context object would have all the required headers already injected.
//
// If any headers are missing or malformed, they will be ignored.
// Malformed headers will be logged if InitGlobalTracer was last called with a
// non-nil logger.
func StartSpanFromHTTPContext(ctx context.Context, name string) (context.Context, *Span) {
	logger := globalTracer.getLogger()
	span := newSpan(nil, name, SpanTypeServer)
	defer func() {
		onCreateServerSpan(span)
		span.onStart()
	}()
	ctx = opentracing.ContextWithSpan(ctx, span)
	if str, ok := httpbp.GetHeader(ctx, httpbp.TraceIDContextKey); ok {
		if id, err := strconv.ParseUint(str, 10, 64); err != nil {
			logger(fmt.Sprintf(
				"Malformed trace id in http ctx: %q, %v",
				str,
				err,
			))
		} else {
			span.trace.traceID = id
		}
	}
	if str, ok := httpbp.GetHeader(ctx, httpbp.SpanIDContextKey); ok {
		if id, err := strconv.ParseUint(str, 10, 64); err != nil {
			logger(fmt.Sprintf(
				"Malformed parent id in http ctx: %q, %v",
				str,
				err,
			))
		} else {
			span.trace.parentID = id
		}
	}
	if str, ok := httpbp.GetHeader(ctx, httpbp.SpanFlagsContextKey); ok {
		if flags, err := strconv.ParseInt(str, 10, 64); err != nil {
			logger(fmt.Sprintf(
				"Malformed flags in http ctx: %q, %v",
				str,
				err,
			))
		} else {
			span.trace.flags = flags
		}
	}
	str, ok := httpbp.GetHeader(ctx, httpbp.SpanSampledContextKey)
	sampled := ok && str == thriftbp.HeaderTracingSampledTrue
	span.trace.sampled = sampled

	return ctx, span
}
