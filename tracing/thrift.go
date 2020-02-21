package tracing

import (
	"context"
	"fmt"
	"strconv"

	"github.com/apache/thrift/lib/go/thrift"
	opentracing "github.com/opentracing/opentracing-go"

	"github.com/reddit/baseplate.go/set"
	"github.com/reddit/baseplate.go/thriftbp"
)

// StartSpanFromThriftContext creates a server span from thrift context object.
//
// This span would usually be used as the span of the whole thrift endpoint
// handler, and the parent of the child-spans.
//
// Caller should pass in the context object they got from thrift library,
// which would have all the required headers already injected.
//
// Please note that "Sampled" header is default to false according to baseplate
// spec, so if the context object doesn't have headers injected correctly,
// this span (and all its child-spans) will never be sampled,
// unless debug flag was set explicitly later.
//
// If any of the tracing related thrift header is present but malformed,
// it will be ignored.
// The error will also be logged if InitGlobalTracer was last called with a
// non-nil logger.
// Absent tracing related headers are always silently ignored.
func StartSpanFromThriftContext(ctx context.Context, name string) (context.Context, *Span) {
	logger := globalTracer.getLogger()
	span := newSpan(nil, name, SpanTypeServer)
	defer func() {
		onCreateServerSpan(span)
		span.onStart()
	}()
	ctx = opentracing.ContextWithSpan(ctx, span)
	if str, ok := thrift.GetHeader(ctx, thriftbp.HeaderTracingTrace); ok {
		if id, err := strconv.ParseUint(str, 10, 64); err != nil {
			if logger != nil {
				logger(fmt.Sprintf(
					"Malformed trace id in thrift ctx: %q, %v",
					str,
					err,
				))
			}
		} else {
			span.trace.traceID = id
		}
	}
	if str, ok := thrift.GetHeader(ctx, thriftbp.HeaderTracingSpan); ok {
		if id, err := strconv.ParseUint(str, 10, 64); err != nil {
			if logger != nil {
				logger(fmt.Sprintf(
					"Malformed span id in thrift ctx: %q, %v",
					str,
					err,
				))
			}
		} else {
			span.trace.parentID = id
		}
	}
	if str, ok := thrift.GetHeader(ctx, thriftbp.HeaderTracingFlags); ok {
		if flags, err := strconv.ParseInt(str, 10, 64); err != nil {
			if logger != nil {
				logger(fmt.Sprintf(
					"Malformed flags in thrift ctx: %q, %v",
					str,
					err,
				))
			}
		} else {
			span.trace.flags = flags
		}
	}
	str, ok := thrift.GetHeader(ctx, thriftbp.HeaderTracingSampled)
	sampled := ok && str == thriftbp.HeaderTracingSampledTrue
	span.trace.sampled = sampled

	return ctx, span
}

// CreateThriftContextFromSpan injects span info into a context object that can
// be used in thrift client code.
//
// Caller should first create a client child-span for the thrift call as usual,
// then use that span and the parent context object with this call,
// then use the returned context object in the thrift call.
// Something like:
//
//     span, clientCtx := opentracing.StartSpanFromContext(
//       ctx,
//       "myCall",
//       tracing.SpanTypeOption{Type: SpanTypeClient},
//     )
//     result, err := client.MyCall(clientCtx, arg1, arg2)
//     // Or: span.Stop(ctx, err)
//     span.FinishWithOptions(tracing.FinishOptions{
//       Ctx: ctx,
//       Err: err,
//     }.Convert())
func CreateThriftContextFromSpan(ctx context.Context, span *Span) context.Context {
	headers := set.StringSliceToSet(thrift.GetWriteHeaderList(ctx))

	ctx = thrift.SetHeader(
		ctx,
		thriftbp.HeaderTracingTrace,
		strconv.FormatUint(span.trace.traceID, 10),
	)
	headers.Add(thriftbp.HeaderTracingTrace)

	ctx = thrift.SetHeader(
		ctx,
		thriftbp.HeaderTracingSpan,
		strconv.FormatUint(span.trace.spanID, 10),
	)
	headers.Add(thriftbp.HeaderTracingSpan)

	ctx = thrift.SetHeader(
		ctx,
		thriftbp.HeaderTracingFlags,
		strconv.FormatInt(span.trace.flags, 10),
	)
	headers.Add(thriftbp.HeaderTracingFlags)

	if span.trace.parentID != 0 {
		ctx = thrift.SetHeader(
			ctx,
			thriftbp.HeaderTracingParent,
			strconv.FormatUint(span.trace.parentID, 10),
		)
		headers.Add(thriftbp.HeaderTracingParent)
	} else {
		headers.Remove(thriftbp.HeaderTracingParent)
	}

	if span.trace.sampled {
		ctx = thrift.SetHeader(
			ctx,
			thriftbp.HeaderTracingSampled,
			thriftbp.HeaderTracingSampledTrue,
		)
		headers.Add(thriftbp.HeaderTracingSampled)
	} else {
		headers.Remove(thriftbp.HeaderTracingSampled)
	}

	ctx = thrift.SetWriteHeaderList(ctx, headers.ToSlice())

	return ctx
}
