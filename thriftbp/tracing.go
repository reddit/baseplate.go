package thriftbp

import (
	"context"
	"strconv"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/reddit/baseplate.go/tracing"
)

// CreateThriftContextFromSpan injects span info into a context object that can
// be used in thrift client code.  If you are using a client pool created using
// thriftbp.NewBaseplateClientPool, all of your thrift calls will already be
// call this automatically, so there is no need to use it directly.
//
// Caller should first create a client child-span for the thrift call as usual,
// then use that span and the parent context object with this call,
// then use the returned context object in the thrift call.
// Something like:
//
//     span, clientCtx := opentracing.StartSpanFromContext(
//       ctx,
//       "myCall",
//       tracing.SpanTypeOption{Type: tracing.SpanTypeClient},
//     )
//     clientCtx = thriftbp.CreateThriftContextFromSpan(clientCtx, tracing.AsSpan(span))
//     result, err := client.MyCall(clientCtx, arg1, arg2)
//     span.FinishWithOptions(tracing.FinishOptions{
//       Ctx: clientCtx,
//       Err: err,
//     }.Convert())
func CreateThriftContextFromSpan(ctx context.Context, span *tracing.Span) context.Context {
	headers := thrift.GetWriteHeaderList(ctx)

	ctx = thrift.SetHeader(
		ctx,
		HeaderTracingTrace,
		span.TraceID(),
	)
	headers = append(headers, HeaderTracingTrace)

	ctx = thrift.SetHeader(
		ctx,
		HeaderTracingSpan,
		span.ID(),
	)
	headers = append(headers, HeaderTracingSpan)

	ctx = thrift.SetHeader(
		ctx,
		HeaderTracingFlags,
		strconv.FormatInt(span.Flags(), 10),
	)
	headers = append(headers, HeaderTracingFlags)

	if span.ParentID() != "" {
		ctx = thrift.SetHeader(
			ctx,
			HeaderTracingParent,
			span.ParentID(),
		)
		headers = append(headers, HeaderTracingParent)
	} else {
		ctx = thrift.UnsetHeader(ctx, HeaderTracingParent)
	}

	if span.Sampled() {
		ctx = thrift.SetHeader(
			ctx,
			HeaderTracingSampled,
			HeaderTracingSampledTrue,
		)
		headers = append(headers, HeaderTracingSampled)
	} else {
		ctx = thrift.UnsetHeader(ctx, HeaderTracingSampled)
	}

	ctx = thrift.SetWriteHeaderList(ctx, headers)

	return ctx
}
