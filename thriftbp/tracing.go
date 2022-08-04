package thriftbp

import (
	"context"
	"strconv"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/reddit/baseplate.go/tracing"
	"github.com/reddit/baseplate.go/transport"
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
//	span, clientCtx := opentracing.StartSpanFromContext(
//	  ctx,
//	  "myCall",
//	  tracing.SpanTypeOption{Type: tracing.SpanTypeClient},
//	)
//	clientCtx = thriftbp.CreateThriftContextFromSpan(clientCtx, tracing.AsSpan(span))
//	result, err := client.MyCall(clientCtx, arg1, arg2)
//	span.FinishWithOptions(tracing.FinishOptions{
//	  Ctx: clientCtx,
//	  Err: err,
//	}.Convert())
func CreateThriftContextFromSpan(ctx context.Context, span *tracing.Span) context.Context {
	headers := thrift.GetWriteHeaderList(ctx)

	ctx = thrift.SetHeader(
		ctx,
		transport.HeaderTracingTrace,
		span.TraceID(),
	)
	headers = append(headers, transport.HeaderTracingTrace)

	ctx = thrift.SetHeader(
		ctx,
		transport.HeaderTracingSpan,
		span.ID(),
	)
	headers = append(headers, transport.HeaderTracingSpan)

	ctx = thrift.SetHeader(
		ctx,
		transport.HeaderTracingFlags,
		strconv.FormatInt(span.Flags(), 10),
	)
	headers = append(headers, transport.HeaderTracingFlags)

	if span.ParentID() != "" {
		ctx = thrift.SetHeader(
			ctx,
			transport.HeaderTracingParent,
			span.ParentID(),
		)
		headers = append(headers, transport.HeaderTracingParent)
	} else {
		ctx = thrift.UnsetHeader(ctx, transport.HeaderTracingParent)
	}

	if span.Sampled() {
		ctx = thrift.SetHeader(
			ctx,
			transport.HeaderTracingSampled,
			transport.HeaderTracingSampledTrue,
		)
		headers = append(headers, transport.HeaderTracingSampled)
	} else {
		ctx = thrift.UnsetHeader(ctx, transport.HeaderTracingSampled)
	}

	ctx = thrift.SetWriteHeaderList(ctx, headers)

	return ctx
}
