package thriftbp

import (
	"context"
	"strconv"
	"time"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/reddit/baseplate.go/edgecontext"
	"github.com/reddit/baseplate.go/errorsbp"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/tracing"
)

var (
	_ thrift.ProcessorMiddleware = ExtractDeadlineBudget
)

// DefaultProcessorMiddlewaresArgs are the args to be passed into
// BaseplateDefaultProcessorMiddlewares function to create default processor
// middlewares.
type DefaultProcessorMiddlewaresArgs struct {
	// The edge context implementation. Required.
	EdgeContextImpl *edgecontext.Impl

	// Suppress some of the errors returned by the server before sending them to
	// the server span.
	//
	// Based on Baseplate spec, the errors defined in your thrift IDL are not
	// treated as errors, and should be suppressed here. So in most cases that's
	// what the service developer should implement as the Suppressor here.
	//
	// Note that this suppressor only affects the errors send to the span. It
	// won't affect the errors returned to the client.
	//
	// This is optional. If it's not set none of the errors will be suppressed.
	ErrorSpanSuppressor errorsbp.Suppressor
}

// BaseplateDefaultProcessorMiddlewares returns the default processor
//  middlewares that should be used by a baseplate Thrift service.
//
// Currently they are (in order):
//
// 1. ExtractDeadlineBudget
//
// 2. InjectServerSpan
//
// 3. InjectEdgeContext
func BaseplateDefaultProcessorMiddlewares(args DefaultProcessorMiddlewaresArgs) []thrift.ProcessorMiddleware {
	return []thrift.ProcessorMiddleware{
		ExtractDeadlineBudget,
		InjectServerSpan(args.ErrorSpanSuppressor),
		InjectEdgeContext(args.EdgeContextImpl),
	}
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
func StartSpanFromThriftContext(ctx context.Context, name string) (context.Context, *tracing.Span) {
	var headers tracing.Headers
	var sampled bool

	if str, ok := thrift.GetHeader(ctx, HeaderTracingTrace); ok {
		headers.TraceID = str
	}
	if str, ok := thrift.GetHeader(ctx, HeaderTracingSpan); ok {
		headers.SpanID = str
	}
	if str, ok := thrift.GetHeader(ctx, HeaderTracingFlags); ok {
		headers.Flags = str
	}
	if str, ok := thrift.GetHeader(ctx, HeaderTracingSampled); ok {
		sampled = str == HeaderTracingSampledTrue
		headers.Sampled = &sampled
	}

	return tracing.StartSpanFromHeaders(ctx, name, headers)
}

// InjectServerSpan implements thrift.ProcessorMiddleware and injects a server
// span into the `next` context.
//
// Starts the server span before calling the `next` TProcessorFunction and stops
// the span after it finishes.
// If the function returns an error that's not suppressed by the suppressor,
// that will be passed to span.Stop.
//
// Note, the span will be created according to tracing related headers already
// being set on the context object.
// These should be automatically injected by your thrift.TSimpleServer.
func InjectServerSpan(suppressor errorsbp.Suppressor) thrift.ProcessorMiddleware {
	return func(name string, next thrift.TProcessorFunction) thrift.TProcessorFunction {
		return thrift.WrappedTProcessorFunction{
			Wrapped: func(ctx context.Context, seqID int32, in, out thrift.TProtocol) (success bool, err thrift.TException) {
				ctx, span := StartSpanFromThriftContext(ctx, name)
				defer func() {
					span.FinishWithOptions(tracing.FinishOptions{
						Ctx: ctx,
						Err: suppressor.Wrap(err),
					}.Convert())
				}()

				return next.Process(ctx, seqID, in, out)
			},
		}
	}
}

// InitializeEdgeContext sets an edge request context created from the Thrift
// headers set on the context onto the context and configures Thrift to forward
// the edge requent context header on any Thrift calls made by the server.
func InitializeEdgeContext(ctx context.Context, impl *edgecontext.Impl) context.Context {
	header, ok := thrift.GetHeader(ctx, HeaderEdgeRequest)
	if !ok {
		return ctx
	}

	ec, err := edgecontext.FromHeader(header, impl)
	if err != nil {
		log.Error("Error while parsing EdgeRequestContext: " + err.Error())
		return ctx
	}
	if ec == nil {
		return ctx
	}

	return edgecontext.SetEdgeContext(ctx, ec)
}

// InjectEdgeContext returns a ProcessorMiddleware that injects an edge request
// context created from the Thrift headers set on the context into the `next`
// thrift.TProcessorFunction.
//
// Note, this depends on the edge context headers already being set on the
// context object.  These should be automatically injected by your
// thrift.TSimpleServer.
func InjectEdgeContext(impl *edgecontext.Impl) thrift.ProcessorMiddleware {
	return func(name string, next thrift.TProcessorFunction) thrift.TProcessorFunction {
		return thrift.WrappedTProcessorFunction{
			Wrapped: func(ctx context.Context, seqID int32, in, out thrift.TProtocol) (bool, thrift.TException) {
				ctx = InitializeEdgeContext(ctx, impl)
				return next.Process(ctx, seqID, in, out)
			},
		}
	}
}

// ExtractDeadlineBudget is the server middleware implementing Phase 1 of
// Baseplate deadline propagation.
//
// It only sets the timeout if the passed in deadline is at least 1ms.
func ExtractDeadlineBudget(name string, next thrift.TProcessorFunction) thrift.TProcessorFunction {
	return thrift.WrappedTProcessorFunction{
		Wrapped: func(ctx context.Context, seqID int32, in, out thrift.TProtocol) (bool, thrift.TException) {
			if s, ok := thrift.GetHeader(ctx, HeaderDeadlineBudget); ok {
				v, err := strconv.ParseInt(s, 10, 64)
				if err == nil && v >= 1 {
					var cancel context.CancelFunc
					ctx, cancel = context.WithTimeout(ctx, time.Millisecond*time.Duration(v))
					defer cancel()
				}
			}
			return next.Process(ctx, seqID, in, out)
		},
	}
}
