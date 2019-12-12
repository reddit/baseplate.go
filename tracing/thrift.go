package tracing

import (
	"context"
	"strconv"

	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/set"
	"github.com/reddit/baseplate.go/thriftbp"

	"github.com/apache/thrift/lib/go/thrift"
	zipkin "github.com/openzipkin/zipkin-go"
	"github.com/openzipkin/zipkin-go/model"
)

// StartSpanFromThriftContext creates a span from the thrift context object.
//
// This span would usually be used as the span of the whole thrift endpoint
// handler, and the parent of the subspans.
//
// Caller should pass in the context object they got from thrift library,
// which would have all the required headers already injected.
//
// Please note that "Sampled" header is default to false according to baseplate
// spec, so if the context object doesn't have headers injected correctly,
// this span (and all its subspans) will never be sampled.
//
// If any of the tracing related thrift header is present but malformed,
// it will be ignored.
// The error will be logged.
// Absent tracing related headers are always silently ignored.
func StartSpanFromThriftContext(ctx context.Context, optName string) zipkin.Span {
	var parentCtx model.SpanContext
	if str, ok := thrift.GetHeader(ctx, thriftbp.HeaderTracingTrace); ok {
		if id, err := strconv.ParseUint(str, 10, 64); err != nil {
			log.Errorf("Malformed trace id in thrift ctx: %q, %v", str, err)
		} else {
			parentCtx.TraceID = model.TraceID{
				Low: id,
			}
		}
	}
	if str, ok := thrift.GetHeader(ctx, thriftbp.HeaderTracingSpan); ok {
		if id, err := strconv.ParseUint(str, 10, 64); err != nil {
			log.Errorf("Malformed span id in thrift ctx: %q, %v", str, err)
		} else {
			parentCtx.ID = model.ID(id)
		}
	}
	str, ok := thrift.GetHeader(ctx, thriftbp.HeaderTracingSampled)
	sampled := ok && str == thriftbp.HeaderTracingSampledTrue
	parentCtx.Sampled = &sampled

	return Tracer.StartSpan(optName, zipkin.Parent(parentCtx))
}

// CreateThriftContextFromSpan injects span info into a context object that can
// be used in thrift client code.
//
// Caller should first create a span or subspan for the thrift call as usual,
// then use that span and the parent context object with this call,
// then use the returned context object in the thrift call.
// Something like:
//
//     span := tracing.StartSpanFromParent("myCall", parentSpan)
//     clientCtx := tracing.CreateThriftContextFromSpan(ctx, span)
//     result, err := client.MyCall(clientCtx, arg1, arg2)
//     tracing.EndSpan(span)
func CreateThriftContextFromSpan(ctx context.Context, span zipkin.Span) context.Context {
	zipkinCtx := span.Context()
	headers := set.StringSliceToSet(thrift.GetWriteHeaderList(ctx))

	ctx = thrift.SetHeader(
		ctx,
		thriftbp.HeaderTracingTrace,
		strconv.FormatUint(zipkinCtx.TraceID.Low, 10),
	)
	headers.Add(thriftbp.HeaderTracingTrace)

	ctx = thrift.SetHeader(
		ctx,
		thriftbp.HeaderTracingSpan,
		strconv.FormatUint(uint64(zipkinCtx.ID), 10),
	)
	headers.Add(thriftbp.HeaderTracingSpan)

	if zipkinCtx.ParentID != nil {
		ctx = thrift.SetHeader(
			ctx,
			thriftbp.HeaderTracingParent,
			strconv.FormatUint(uint64(*zipkinCtx.ParentID), 10),
		)
		headers.Add(thriftbp.HeaderTracingParent)
	}

	if zipkinCtx.Sampled != nil {
		if *zipkinCtx.Sampled {
			ctx = thrift.SetHeader(
				ctx,
				thriftbp.HeaderTracingSampled,
				thriftbp.HeaderTracingSampledTrue,
			)
		} else {
			ctx = thrift.SetHeader(
				ctx,
				thriftbp.HeaderTracingSampled,
				"",
			)
		}
		headers.Add(thriftbp.HeaderTracingSampled)
	}

	ctx = thrift.SetWriteHeaderList(ctx, headers.ToSlice())

	return ctx
}
