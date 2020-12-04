package log

import (
	"context"

	"github.com/reddit/zap"
)

type contextKeyType struct{}

var contextKey contextKeyType

// logger keys for attached data.
const (
	traceIDKey = "trace_id"
)

// AttachArgs are used to create loggers to be attached to context object with
// pre-filled key-value pairs.
//
// All zero value fields will be ignored and only non-zero values will be
// attached.
//
// AdditionalPairs are provided to add any free form, additional key-value pairs
// you want to attach to all logs from the same context object.
type AttachArgs struct {
	TraceID uint64

	AdditionalPairs map[string]interface{}
}

// Attach attaches a logger with data extracted from args into the context
// object.
func Attach(ctx context.Context, args AttachArgs) context.Context {
	// Number of non-AdditionalPairs fields in AttachArgs struct.
	const additional = 1
	kv := make([]interface{}, 0, len(args.AdditionalPairs)*2+additional)

	if args.TraceID != 0 {
		kv = append(kv, zap.Uint64(traceIDKey, args.TraceID))
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
//     log.C(ctx).Errorw("Something went wrong!", "err", err)
//
// The return value is guaranteed to be non-nil.
func C(ctx context.Context) *zap.SugaredLogger {
	if logger, ok := ctx.Value(contextKey).(*zap.SugaredLogger); ok && logger != nil {
		return logger
	}
	return globalLogger
}
