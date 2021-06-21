package redisbp

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-redis/redis/v8"
	opentracing "github.com/opentracing/opentracing-go"

	"github.com/reddit/baseplate.go/errorsbp"
	"github.com/reddit/baseplate.go/tracing"
)

// SpanHook is a redis.Hook for wrapping Redis commands and pipelines
// in Client Spans and metrics.
type SpanHook struct {
	ClientName string
}

var _ redis.Hook = SpanHook{}

// BeforeProcess starts a client Span before processing a Redis command and
// starts a timer to record how long the command took.
func (h SpanHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	return h.startChildSpan(ctx, cmd.Name()), nil
}

// AfterProcess ends the client Span started by BeforeProcess, publishes the
// time the Redis command took to complete, and a metric indicating whether the
// command was a "success" or "fail"
func (h SpanHook) AfterProcess(ctx context.Context, cmd redis.Cmder) error {
	h.endChildSpan(ctx, cmd.Err())
	// NOTE: returning non-nil error from the hook changes the error the caller gets.
	// for this particular case if we return cmd.Err(), it will not change the client error,
	// but anyway it's not necessary
	// see: https://github.com/go-redis/redis/blob/v8.10.0/redis.go#L60
	return nil
}

// BeforeProcessPipeline starts a client span before processing a Redis pipeline
// and starts a timer to record how long the pipeline took.
func (h SpanHook) BeforeProcessPipeline(ctx context.Context, cmds []redis.Cmder) (context.Context, error) {
	return h.startChildSpan(ctx, "pipeline"), nil
}

// AfterProcessPipeline ends the client span started by BeforeProcessPipeline,
// publishes the time the Redis pipeline took to complete, and a metric
// indicating whether the pipeline was a "success" or "fail"
func (h SpanHook) AfterProcessPipeline(ctx context.Context, cmds []redis.Cmder) error {
	var errs errorsbp.Batch
	for _, cmd := range cmds {
		if !errors.Is(cmd.Err(), redis.Nil) {
			errs.Add(cmd.Err())
		}
	}
	h.endChildSpan(ctx, errs.Compile())
	// NOTE: returning non-nil error from the hook changes the error the caller gets, and that's something we want to avoid.
	// see: https://github.com/go-redis/redis/blob/v8.10.0/redis.go#L101
	return nil
}

func (h SpanHook) startChildSpan(ctx context.Context, cmdName string) context.Context {
	name := fmt.Sprintf("%s.%s", h.ClientName, cmdName)
	_, ctx = opentracing.StartSpanFromContext(
		ctx,
		name,
		tracing.SpanTypeOption{Type: tracing.SpanTypeClient},
	)
	return ctx
}

func (h SpanHook) endChildSpan(ctx context.Context, err error) {
	if span := opentracing.SpanFromContext(ctx); span != nil {
		span.FinishWithOptions(tracing.FinishOptions{
			Ctx: ctx,
			Err: err,
		}.Convert())
	}
}
