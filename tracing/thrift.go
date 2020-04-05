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

// Headers is the argument struct for starting a Span from upstream headers.
type Headers struct {
	// TraceID is the trace ID passed via upstream headers.
	TraceID string

	// SpanID is the span ID passed via upstream headers.
	SpanID string

	// Flags is the flags int passed via upstream headers as a string.
	Flags string

	// Sampled is whether this span was sampled by the upstream caller.  Uses
	// a pointer to a bool so it can distinguish between set/not-set.
	Sampled *bool
}

// StartSpanFromHeaders creates a server span from the passed in Headers.
//
// Please note that "Sampled" header is default to false according to baseplate
// spec, so if the headers are incorrect, this span (and all its child-spans)
// will never be sampled, unless debug flag was set explicitly later.
//
// If any headers are missing or malformed, they will be ignored.
// Malformed headers will be logged if InitGlobalTracer was last called with a
// non-nil logger.
func StartSpanFromHeaders(ctx context.Context, name string, headers Headers) (context.Context, *Span) {
	logger := globalTracer.getLogger()
	span := newSpan(nil, name, SpanTypeServer)
	defer func() {
		onCreateServerSpan(span)
		span.onStart()
	}()

	ctx = opentracing.ContextWithSpan(ctx, span)

	if headers.TraceID != "" {
		if id, err := strconv.ParseUint(headers.TraceID, 10, 64); err != nil {
			logger(fmt.Sprintf(
				"Malformed trace id in http ctx: %q, %v",
				headers.TraceID,
				err,
			))
		} else {
			span.trace.traceID = id
		}
	}

	if headers.SpanID != "" {
		if id, err := strconv.ParseUint(headers.SpanID, 10, 64); err != nil {
			logger(fmt.Sprintf(
				"Malformed parent id in http ctx: %q, %v",
				headers.SpanID,
				err,
			))
		} else {
			span.trace.parentID = id
		}
	}

	if headers.Flags != "" {
		if flags, err := strconv.ParseInt(headers.Flags, 10, 64); err != nil {
			logger(fmt.Sprintf(
				"Malformed flags in http ctx: %q, %v",
				headers.Flags,
				err,
			))
		} else {
			span.trace.flags = flags
		}
	}

	if headers.Sampled != nil {
		span.trace.sampled = *headers.Sampled
	}

	return ctx, span
}

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
	var headers Headers
	var sampled bool

	if str, ok := thrift.GetHeader(ctx, thriftbp.HeaderTracingTrace); ok {
		headers.TraceID = str
	}
	if str, ok := thrift.GetHeader(ctx, thriftbp.HeaderTracingSpan); ok {
		headers.SpanID = str
	}
	if str, ok := thrift.GetHeader(ctx, thriftbp.HeaderTracingFlags); ok {
		headers.Flags = str
	}
	if str, ok := thrift.GetHeader(ctx, thriftbp.HeaderTracingSampled); ok {
		sampled = str == thriftbp.HeaderTracingSampledTrue
		headers.Sampled = &sampled
	}

	return StartSpanFromHeaders(ctx, name, headers)
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
