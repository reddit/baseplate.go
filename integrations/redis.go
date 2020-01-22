package integrations

import (
	"context"
	"fmt"

	"github.com/go-redis/redis/v7"

	"github.com/reddit/baseplate.go/batcherror"
	"github.com/reddit/baseplate.go/tracing"
)

// RedisSpanHook is a redis.Hook for wrapping Redis commands and pipelines
// in Client Spans and metrics.
type RedisSpanHook struct {
	ClientName string
}

var _ redis.Hook = RedisSpanHook{}

// BeforeProcess starts a client Span before processing a Redis command and
// starts a timer to record how long the command took.
func (h RedisSpanHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	return h.startChildSpan(ctx, cmd.Name()), nil
}

// AfterProcess ends the client Span started by BeforeProcess, publishes the
// time the Redis command took to complete, and a metric indicating whether the
// command was a "success" or "fail"
func (h RedisSpanHook) AfterProcess(ctx context.Context, cmd redis.Cmder) error {
	return h.endChildSpan(ctx, cmd.Err())
}

// BeforeProcessPipeline starts a client span before processing a Redis pipeline
// and starts a timer to record how long the pipeline took.
func (h RedisSpanHook) BeforeProcessPipeline(ctx context.Context, cmds []redis.Cmder) (context.Context, error) {
	return h.startChildSpan(ctx, "pipeline"), nil
}

// AfterProcessPipeline ends the client span started by BeforeProcessPipeline,
// publishes the time the Redis pipeline took to complete, and a metric
// indicating whether the pipeline was a "success" or "fail"
func (h RedisSpanHook) AfterProcessPipeline(ctx context.Context, cmds []redis.Cmder) error {
	var errs batcherror.BatchError
	for _, cmd := range cmds {
		errs.Add(cmd.Err())
	}
	return h.endChildSpan(ctx, errs.Compile())
}

func (h RedisSpanHook) startChildSpan(ctx context.Context, cmdName string) context.Context {
	span := tracing.GetServerSpan(ctx)
	if span == nil {
		return ctx
	}
	name := fmt.Sprintf("%s.%s", h.ClientName, cmdName)
	ctx, _ = span.CreateClientChildForContext(ctx, name)
	return ctx
}

func (h RedisSpanHook) endChildSpan(ctx context.Context, err error) error {
	if span := tracing.GetChildSpan(ctx); span != nil {
		return span.End(ctx, err)
	}
	return nil
}
