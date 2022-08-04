package log

import (
	"context"
	"fmt"

	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"

	"github.com/reddit/baseplate.go/detach"
	//lint:ignore SA1019 This library is internal only, not actually deprecated
	"github.com/reddit/baseplate.go/internalv2compat"
)

type contextKeyType struct{}

var contextKey contextKeyType

func init() {
	copyContext := func(dst, src context.Context) context.Context {
		if logger, ok := src.Value(contextKey).(*zap.SugaredLogger); ok && logger != nil {
			dst = context.WithValue(dst, contextKey, logger)
		}
		return dst
	}
	detach.Register(detach.Hooks{
		Inline: copyContext,
		Async: func(dst, src context.Context, next func(ctx context.Context)) {
			next(copyContext(dst, src))
		},
	})
}

// logger keys for attached data.
const (
	traceIDKey = "traceID"
)

// AttachArgs are used to create loggers and sentry hubs to be attached to
// context object with pre-filled key-value pairs.
//
// All zero value fields will be ignored and only non-zero values will be
// attached.
//
// AdditionalPairs are provided to add any free form, additional key-value pairs
// you want to attach to all logs and sentry reports from the same context
// object.
type AttachArgs struct {
	TraceID string

	AdditionalPairs map[string]interface{}
}

// Attach attaches a logger and sentry hub with data extracted from args into
// the context object.
func Attach(ctx context.Context, args AttachArgs) context.Context {
	// create and attach the sentry hub
	hub := sentry.GetHubFromContext(ctx)
	if hub == nil {
		hub = sentry.CurrentHub()
	}
	hub = hub.Clone()
	hub.ConfigureScope(func(scope *sentry.Scope) {
		if args.TraceID != "" {
			scope.SetTag("trace_id", args.TraceID)
		}
		for k, v := range args.AdditionalPairs {
			scope.SetTag(k, fmt.Sprintf("%v", v))
		}
	})
	ctx = context.WithValue(ctx, sentry.HubContextKey, hub)

	// create and attach the logger
	const additional = 1 // Number of non-AdditionalPairs fields in AttachArgs struct.
	kv := make([]interface{}, 0, len(args.AdditionalPairs)*2+additional)
	if args.TraceID != "" {
		kv = append(kv, zap.String(traceIDKey, args.TraceID))
	}
	for k, v := range args.AdditionalPairs {
		kv = append(kv, k, v)
	}
	logger := C(ctx)
	if len(kv) == 0 {
		// We can also just return ctx directly here without attaching,
		// but attaching the value again will make log.C(ctx) faster,
		// which is usually used a lot more than other values from the context
		// object.
		return context.WithValue(ctx, contextKey, logger)
	}
	return context.WithValue(ctx, contextKey, logger.With(kv...))
}

// C is short for Context.
//
// It extract the logger attached to the current context object,
// and fallback to the global logger if none is found.
//
// When you have a context object and want to do logging,
// you should always use this one instead of the global one.
// For example:
//
//	log.C(ctx).Errorw("Something went wrong!", "err", err)
//
// The return value is guaranteed to be non-nil.
func C(ctx context.Context) *zap.SugaredLogger {
	if logger, ok := ctx.Value(contextKey).(*zap.SugaredLogger); ok && logger != nil {
		return logger
	}
	return internalv2compat.GlobalLogger()
}
