package thriftbp

import (
	"context"
	"strconv"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/reddit/baseplate.go/tracing"
)

// CreateThriftContextFromSpan injects span info into a context object that can
// be used in thrift client code.  If you are using a client pool created using
// thriftclient.NewBaseplateClientPool, all of your thrift calls will already be
// call this automatically, so there is no need to use it directly.
//
// Caller should first create a client child-span for the thrift call as usual,
// then use that span and the parent context object with this call,
// then use the returned context object in the thrift call.
// Something like:
//
//		 var s opentracing.Span
//     span, clientCtx := opentracing.StartSpanFromContext(
//       ctx,
//       "myCall",
//       tracing.SpanTypeOption{Type: tracing.SpanTypeClient},
//     )
//		 span := tracing.AsSpan(s)
//		 ctx = thriftbp.CreateThriftContextFromSpan(ctx, span)
//     result, err := client.MyCall(clientCtx, arg1, arg2)
//     span.FinishWithOptions(tracing.FinishOptions{
//       Ctx: ctx,
//       Err: err,
//     }.Convert())
func CreateThriftContextFromSpan(ctx context.Context, span *tracing.Span) context.Context {
	headers := thrift.GetWriteHeaderList(ctx)

	ctx = thrift.SetHeader(
		ctx,
		HeaderTracingTrace,
		strconv.FormatUint(span.TraceID(), 10),
	)
	headers = append(headers, HeaderTracingTrace)

	ctx = thrift.SetHeader(
		ctx,
		HeaderTracingSpan,
		strconv.FormatUint(span.ID(), 10),
	)
	headers = append(headers, HeaderTracingSpan)

	ctx = thrift.SetHeader(
		ctx,
		HeaderTracingFlags,
		strconv.FormatInt(span.Flags(), 10),
	)
	headers = append(headers, HeaderTracingFlags)

	if span.ParentID() != 0 {
		ctx = thrift.SetHeader(
			ctx,
			HeaderTracingParent,
			strconv.FormatUint(span.ParentID(), 10),
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
