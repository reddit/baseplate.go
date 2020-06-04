package thriftbp

import (
	"context"
	"strconv"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	retry "github.com/avast/retry-go"
	opentracing "github.com/opentracing/opentracing-go"

	"github.com/reddit/baseplate.go/edgecontext"
	"github.com/reddit/baseplate.go/retrybp"
	"github.com/reddit/baseplate.go/tracing"
)

// MonitorClientWrappedSlugSuffix is a suffix to be added to the service slug
// arg of MonitorClient function, in order to distinguish from the spans that
// are the raw client calls..
//
// The MonitorClient with this suffix will have span operation names like:
//
//     service-with-retry.endpointName
//
// Which groups all retries of the same client call together,
// while the MonitorClient without this suffix will have span operation names
// like:
//
//     service.endpointName
const MonitorClientWrappedSlugSuffix = "-with-retry"

// WithDefaultRetryFilters returns a list of retrybp.Filters by appending the
// given filters to the "default" retry filters:
//
// 1. ContextErrorFilter - do not retry on context cancellation/timeout.
//
// 2. UnrecoverableErrorFilter - do not retry errors marked as unrecoverable with
//    retry.Unrecoverable.
//
// 3. PoolExhaustedFilter - do retry on clientpool.PoolExhausted errors.
func WithDefaultRetryFilters(filters ...retrybp.Filter) []retrybp.Filter {
	return append([]retrybp.Filter{
		retrybp.ContextErrorFilter,
		retrybp.UnrecoverableErrorFilter,
		retrybp.PoolExhaustedFilter,
	}, filters...)
}

// DefaultClientMiddlewareArgs is the arg struct for BaseplateDefaultClientMiddlewares.
type DefaultClientMiddlewareArgs struct {
	// ServiceSlug is a short identifier for the thrift service you are creating
	// clients for.  The preferred convention is to take the service's name,
	// remove the 'Service' prefix, if present, and convert from camel case to
	// all lower case, hyphen separated.
	//
	// Examples:
	//
	//     AuthenticationService -> authentication
	//     ImageUploadService -> image-upload
	ServiceSlug string

	// RetryOptions is the list of retry.Options to apply as the defaults for the
	// Retry middleware.
	//
	// This is optional, if it is not set, we will use a single option,
	// retry.Attempts(1).  This sets up the retry middleware but does not
	// automatically retry any requests.  You can set retry behavior per-call by
	// using retrybp.WithOptions.
	RetryOptions []retry.Option
}

// BaseplateDefaultClientMiddlewares returns the default client middlewares that
// should be used by a baseplate service.
//
// Currently they are (in order):
//
// 1. ForwardEdgeRequestContext
//
// 2. MonitorClient with MonitorClientWrappedSlugSuffix - This creates the spans
// from the view of the client that group all retries into a single,
// wrapped span.
//
// 3. Retry(retryOptions) - If retryOptions is empty/nil, default to only
// retry.Attempts(1), this will not actually retry any calls but your client is
// configured to set retry logic per-call using retrybp.WithOptions.
//
// 4. MonitorClient - This creates the spans of the raw client calls.
//
// 5. SetDeadlineBudget
func BaseplateDefaultClientMiddlewares(args DefaultClientMiddlewareArgs) []thrift.ClientMiddleware {
	if len(args.RetryOptions) == 0 {
		args.RetryOptions = []retry.Option{retry.Attempts(1)}
	}
	return []thrift.ClientMiddleware{
		ForwardEdgeRequestContext,
		MonitorClient(args.ServiceSlug + MonitorClientWrappedSlugSuffix),
		Retry(args.RetryOptions...),
		MonitorClient(args.ServiceSlug),
		SetDeadlineBudget,
	}
}

// MonitorClient is a ClientMiddleware that wraps the inner thrift.TClient.Call
// in a thrift client span.
//
// If you are using a thrift ClientPool created by NewBaseplateClientPool,
// this will be included automatically and should not be passed in as a
// ClientMiddleware to NewBaseplateClientPool.
func MonitorClient(service string) thrift.ClientMiddleware {
	prefix := service + "."
	return func(next thrift.TClient) thrift.TClient {
		return thrift.WrappedTClient{
			Wrapped: func(ctx context.Context, method string, args, result thrift.TStruct) (err error) {
				span, ctx := opentracing.StartSpanFromContext(
					ctx,
					prefix+method,
					tracing.SpanTypeOption{Type: tracing.SpanTypeClient},
				)
				ctx = CreateThriftContextFromSpan(ctx, tracing.AsSpan(span))
				defer func() {
					span.FinishWithOptions(tracing.FinishOptions{
						Ctx: ctx,
						Err: err,
					}.Convert())
				}()

				return next.Call(ctx, method, args, result)
			},
		}
	}
}

// ForwardEdgeRequestContext forwards the EdgeRequestContext set on the context
// object to the Thrift service being called if one is set.
//
// If you are using a thrift ClientPool created by NewBaseplateClientPool,
// this will be included automatically and should not be passed in as a
// ClientMiddleware to NewBaseplateClientPool.
func ForwardEdgeRequestContext(next thrift.TClient) thrift.TClient {
	return thrift.WrappedTClient{
		Wrapped: func(ctx context.Context, method string, args, result thrift.TStruct) (err error) {
			if ec, ok := edgecontext.GetEdgeContext(ctx); ok {
				ctx = AttachEdgeRequestContext(ctx, ec)
			}
			return next.Call(ctx, method, args, result)
		},
	}
}

// SetDeadlineBudget is the client middleware implementing Phase 1 of Baseplate
// deadline propogation.
func SetDeadlineBudget(next thrift.TClient) thrift.TClient {
	return thrift.WrappedTClient{
		Wrapped: func(ctx context.Context, method string, args, result thrift.TStruct) error {
			if ctx.Err() != nil {
				// Deadline already passed, no need to even try
				return ctx.Err()
			}

			if deadline, ok := ctx.Deadline(); ok {
				// Round up to the next millisecond.
				// In the scenario that the caller set an 10ms timeout and send the
				// request, by the time we get into this middleware function it's
				// definitely gonna be less than 10ms.
				// If we use round down then we are only gonna send 9 over the wire.
				timeout := deadline.Sub(time.Now()) + time.Millisecond - 1
				ms := timeout.Milliseconds()
				if ms < 1 {
					// Make sure we give it at least 1ms.
					ms = 1
				}
				value := strconv.FormatInt(ms, 10)
				ctx = thrift.SetHeader(ctx, HeaderDeadlineBudget, value)
			}

			return next.Call(ctx, method, args, result)
		},
	}
}

// Retry returns a thrift.ClientMiddleware that can be used to automatically
// retry thrift requests.
func Retry(defaults ...retry.Option) thrift.ClientMiddleware {
	return func(next thrift.TClient) thrift.TClient {
		return thrift.WrappedTClient{
			Wrapped: func(ctx context.Context, method string, args, result thrift.TStruct) error {
				return retrybp.Do(
					ctx,
					func() error {
						return next.Call(ctx, method, args, result)
					},
					defaults...,
				)
			},
		}
	}
}

var (
	_ thrift.ClientMiddleware = ForwardEdgeRequestContext
	_ thrift.ClientMiddleware = SetDeadlineBudget
)
