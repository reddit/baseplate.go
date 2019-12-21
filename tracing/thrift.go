package tracing

import (
	"context"
	"fmt"
	"strconv"

	"github.com/reddit/baseplate.go/set"
	"github.com/reddit/baseplate.go/thriftbp"

	"github.com/apache/thrift/lib/go/thrift"
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
// unless debug flag was set later.
//
// If any of the tracing related thrift header is present but malformed,
// it will be ignored.
// The error will also logged if the global tracer's logger is non-nil.
// Absent tracing related headers are always silently ignored.
func StartSpanFromThriftContext(ctx context.Context, name string) *Span {
	return StartSpanFromThriftContextWithTracer(ctx, name, nil)
}

// StartSpanFromThriftContextWithTracer is the same as
// StartSpanFromThriftContext, except that it uses the passed in tracer instead
// of GlobalTracer.
func StartSpanFromThriftContextWithTracer(ctx context.Context, name string, tracer *Tracer) *Span {
	span := newSpan(tracer, SpanTypeServer)
	tracer = span.tracer
	span.spanType = SpanTypeServer
	if str, ok := thrift.GetHeader(ctx, thriftbp.HeaderTracingTrace); ok {
		if id, err := strconv.ParseUint(str, 10, 64); err != nil {
			if tracer.Logger != nil {
				tracer.Logger(fmt.Sprintf(
					"Malformed trace id in thrift ctx: %q, %v",
					str,
					err,
				))
			}
		} else {
			span.traceID = id
		}
	}
	if str, ok := thrift.GetHeader(ctx, thriftbp.HeaderTracingSpan); ok {
		if id, err := strconv.ParseUint(str, 10, 64); err != nil {
			if tracer.Logger != nil {
				tracer.Logger(fmt.Sprintf(
					"Malformed span id in thrift ctx: %q, %v",
					str,
					err,
				))
			}
		} else {
			span.parentID = id
		}
	}
	if str, ok := thrift.GetHeader(ctx, thriftbp.HeaderTracingFlags); ok {
		if flags, err := strconv.ParseInt(str, 10, 64); err != nil {
			if tracer.Logger != nil {
				tracer.Logger(fmt.Sprintf(
					"Malformed flags in thrift ctx: %q, %v",
					str,
					err,
				))
			}
		} else {
			span.flags = flags
		}
	}
	str, ok := thrift.GetHeader(ctx, thriftbp.HeaderTracingSampled)
	sampled := ok && str == thriftbp.HeaderTracingSampledTrue
	span.sampled = sampled

	return span
}

// CreateThriftContextFromSpan injects span info into a context object that can
// be used in thrift client code.
//
// Caller should first create a client child-span for the thrift call as usual,
// then use that span and the parent context object with this call,
// then use the returned context object in the thrift call.
// Something like:
//
//     span := parentSpan.CreateClientChild("myCall")
//     clientCtx := tracing.CreateThriftContextFromSpan(ctx, span)
//     result, err := client.MyCall(clientCtx, arg1, arg2)
//     span.End(ctx, err)
//
// See Span.ChildAndThriftContext for a shortcut.
func CreateThriftContextFromSpan(ctx context.Context, span *Span) context.Context {
	headers := set.StringSliceToSet(thrift.GetWriteHeaderList(ctx))

	ctx = thrift.SetHeader(
		ctx,
		thriftbp.HeaderTracingTrace,
		strconv.FormatUint(span.traceID, 10),
	)
	headers.Add(thriftbp.HeaderTracingTrace)

	ctx = thrift.SetHeader(
		ctx,
		thriftbp.HeaderTracingSpan,
		strconv.FormatUint(span.spanID, 10),
	)
	headers.Add(thriftbp.HeaderTracingSpan)

	ctx = thrift.SetHeader(
		ctx,
		thriftbp.HeaderTracingFlags,
		strconv.FormatInt(span.flags, 10),
	)
	headers.Add(thriftbp.HeaderTracingFlags)

	if span.parentID != 0 {
		ctx = thrift.SetHeader(
			ctx,
			thriftbp.HeaderTracingParent,
			strconv.FormatUint(span.parentID, 10),
		)
		headers.Add(thriftbp.HeaderTracingParent)
	} else {
		headers.Remove(thriftbp.HeaderTracingParent)
	}

	if span.sampled {
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
