package httpbp

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/reddit/baseplate.go/ecinterface"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/tracing"
)

// AllowHeader is the "Allow" header.  This should be set when returning a
// 405 - Method Not Allowed error.
const AllowHeader = "Allow"

const spanSampledTrue = "1"

// Middleware wraps the given HandlerFunc and returns a new, wrapped, HandlerFunc.
type Middleware func(name string, next HandlerFunc) HandlerFunc

// Wrap wraps the given HandlerFunc with the given Middlewares and returns the
// wrapped HandlerFunc passing the given name to each middleware in the chain.
//
// Middlewares will be called in the order that they are defined:
//
//		1. Middlewares[0]
//		2. Middlewares[1]
//		...
//		N. Middlewares[n]
//
// Wrap is provided for clarity and testing purposes and should not generally be
// called directly.  Instead use one of the provided Handler constructors which
// will Wrap the HandlerFunc you pass it for you.
func Wrap(name string, handle HandlerFunc, middlewares ...Middleware) HandlerFunc {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handle = middlewares[i](name, handle)
	}
	return handle
}

// DefaultMiddlewareArgs provides the arguments for the default, Baseplate
// Middlewares
type DefaultMiddlewareArgs struct {
	// The HeaderTrustHandler to use.
	// If empty, NeverTrustHeaders will be used instead.
	TrustHandler HeaderTrustHandler

	// The logger to be called when edgecontext parsing failed.
	Logger log.Wrapper

	// The edgecontext implementation to use. Optional.
	// If not set, the global one from ecinterface.Get will be used instead.
	EdgeContextImpl ecinterface.Interface
}

// DefaultMiddleware returns a slice of all of the default Middleware for a
// Baseplate HTTP server.
func DefaultMiddleware(args DefaultMiddlewareArgs) []Middleware {
	if args.TrustHandler == nil {
		args.TrustHandler = NeverTrustHeaders{}
	}
	return []Middleware{
		InjectServerSpan(args.TrustHandler),
		InjectEdgeRequestContext(InjectEdgeRequestContextArgs(args)),
	}
}

func isHeaderSet(h http.Header, key string) bool {
	return len(h.Values(key)) > 0
}

// StartSpanFromTrustedRequest starts a server span using the Span headers from
// the given request if the provided HeaderTrustHandler confirms that they can
// be trusted and the Span headers are provided, otherwise it starts a new
// server span.
//
// StartSpanFromTrustedRequest is used by InjectServerSpan and should not
// generally be used directly but is provided for testing purposes or use cases
// that are not covered by Baseplate.
func StartSpanFromTrustedRequest(
	ctx context.Context,
	name string,
	truster HeaderTrustHandler,
	r *http.Request,
) (context.Context, *tracing.Span) {
	var spanHeaders tracing.Headers
	var sampled bool

	if truster.TrustSpan(r) {
		if isHeaderSet(r.Header, TraceIDHeader) {
			spanHeaders.TraceID = r.Header.Get(TraceIDHeader)
		}
		if isHeaderSet(r.Header, SpanIDHeader) {
			spanHeaders.SpanID = r.Header.Get(SpanIDHeader)
		}
		if isHeaderSet(r.Header, SpanFlagsHeader) {
			spanHeaders.Flags = r.Header.Get(SpanFlagsHeader)
		}
		if isHeaderSet(r.Header, SpanSampledHeader) {
			sampled = r.Header.Get(SpanSampledHeader) == spanSampledTrue
			spanHeaders.Sampled = &sampled
		}
	}

	return tracing.StartSpanFromHeaders(ctx, name, spanHeaders)
}

// InjectServerSpan returns a Middleware that will automatically wrap the
// HansderFunc in a new server span and stop the span after the function
// returns.
//
// InjectServerSpan should generally not be used directly, instead use the
// NewBaseplateServer function which will automatically include InjectServerSpan
// as one of the Middlewares to wrap your handlers in.
func InjectServerSpan(truster HeaderTrustHandler) Middleware {
	return func(name string, next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, w http.ResponseWriter, r *http.Request) (err error) {
			ctx, span := StartSpanFromTrustedRequest(ctx, name, truster, r)
			defer func() {
				span.FinishWithOptions(tracing.FinishOptions{
					Ctx: ctx,
					Err: err,
				}.Convert())
			}()

			return next(ctx, w, r)
		}
	}
}

// InitializeEdgeContextFromTrustedRequest initializen an EdgeRequestContext on
// the context object if the provided HeaderTrustHandler confirms that the
// headers can be trusted and the header is set on the request.  If the header
// cannot be trusted and/or the header is not set, then no EdgeRequestContext is
// set on the context object.
//
// InitializeEdgeContextFromTrustedRequest is used by InjectEdgeRequestContext
// and should not generally be used directly but is provided for testing
// purposes or use cases that are not covered by Baseplate.
func InitializeEdgeContextFromTrustedRequest(
	ctx context.Context,
	r *http.Request,
	args InjectEdgeRequestContextArgs,
) context.Context {
	if args.TrustHandler == nil {
		args.TrustHandler = NeverTrustHeaders{}
	}

	if !args.TrustHandler.TrustEdgeContext(r) {
		return ctx
	}

	header, err := decodeEdgeContextHeader(r.Header.Get(EdgeContextHeader))
	if err != nil {
		args.Logger.Log(ctx, "Error while parsing EdgeRequestContext: "+err.Error())
		return ctx
	}
	if args.EdgeContextImpl == nil {
		args.EdgeContextImpl = ecinterface.Get()
	}
	ctx, err = args.EdgeContextImpl.HeaderToContext(ctx, string(header))
	if err != nil {
		args.Logger.Log(ctx, "Error while parsing EdgeRequestContext: "+err.Error())
	}

	return ctx
}

// InjectEdgeRequestContextArgs are the args to be passed into
// InjectEdgeRequestContext function.
type InjectEdgeRequestContextArgs struct {
	// The HeaderTrustHandler to use.
	// If empty, NeverTrustHeaders{} will be used instead.
	TrustHandler HeaderTrustHandler

	// The logger to be called when edgecontext parsing failed.
	Logger log.Wrapper

	// The edgecontext implementation to use. Optional.
	// If not set, the global one from ecinterface.Get will be used instead.
	EdgeContextImpl ecinterface.Interface
}

// InjectEdgeRequestContext returns a Middleware that will automatically parse
// the EdgeRequestContext header from the request headers and attach it to
// the context object if present.
//
// InjectEdgeRequestContext should generally not be used directly, instead use
// the NewBaseplateServer function which will automatically include
// InjectEdgeRequestContext as one of the Middlewares to wrap your handlers in.
func InjectEdgeRequestContext(args InjectEdgeRequestContextArgs) Middleware {
	if args.EdgeContextImpl == nil {
		args.EdgeContextImpl = ecinterface.Get()
	}
	return func(name string, next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
			ctx = InitializeEdgeContextFromTrustedRequest(ctx, r, args)
			return next(ctx, w, r)
		}
	}
}

// SupportedMethods returns a middleware that checks if the request is made
// using one of the given HTTP methods.
//
// Returns a raw, plain text 405 error response if the method is not supported.
// If GET is supported, HEAD will be automatically supported as well.
// Sets the "Allow" header automatically to the methods given.
//
// SupportedMethods should generally not be used directly, instead use the
// NewBaseplateServer function which will automatically include SupportedMethods
// as one of the Middlewares to wrap your handlers in.
func SupportedMethods(method string, additional ...string) Middleware {
	supported := make(map[string]bool, len(additional)+1)
	supported[strings.ToUpper(method)] = true
	for _, m := range additional {
		supported[strings.ToUpper(m)] = true
	}
	if supported[http.MethodGet] {
		supported[http.MethodHead] = true
	}

	allowed := make([]string, len(supported))
	i := 0
	for m := range supported {
		allowed[i] = m
		i++
	}
	sort.Strings(allowed)
	allowedHeader := strings.Join(allowed, ",")

	return func(name string, next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
			if !supported[r.Method] {
				w.Header().Set(AllowHeader, allowedHeader)
				return RawError(
					MethodNotAllowed(),
					fmt.Errorf("method %q is not supported by %q", r.Method, name),
					PlainTextContentType,
				)
			}
			return next(ctx, w, r)
		}
	}
}
