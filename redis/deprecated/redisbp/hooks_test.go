package redisbp_test

import (
	"context"
	"testing"

	"github.com/go-redis/redis/v8"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/reddit/baseplate.go/redis/deprecated/redisbp"
	"github.com/reddit/baseplate.go/thriftbp"
	"github.com/reddit/baseplate.go/tracing"
)

func TestSpanHook(t *testing.T) {
	ctx, _ := thriftbp.StartSpanFromThriftContext(context.Background(), "foo")
	hooks := redisbp.SpanHook{ClientName: "redis"}
	statusCmd := redis.NewStatusCmd(ctx, "ping")
	stringCmd := redis.NewStringCmd(ctx, "get", "1")
	stringCmd.SetErr(redis.Nil)

	t.Run(
		"Before/AfterProcess",
		func(t *testing.T) {
			ctx, err := hooks.BeforeProcess(ctx, statusCmd)
			if err != nil {
				t.Fatalf("Unexpected error: %s", err)
			}
			activeSpan := opentracing.SpanFromContext(ctx)
			if activeSpan == nil {
				t.Fatalf("'activeSpan' is 'nil'")
			}
			if name := tracing.AsSpan(activeSpan).Name(); name != "redis.ping" {
				t.Fatalf("Incorrect span name %q", name)
			}

			if err = hooks.AfterProcess(ctx, statusCmd); err != nil {
				t.Fatalf("Unexpected error: %s", err)
			}
		},
	)

	t.Run(
		"Before/AfterProcessPipeline",
		func(t *testing.T) {
			cmds := []redis.Cmder{statusCmd, stringCmd}
			ctx, err := hooks.BeforeProcessPipeline(ctx, cmds)
			if err != nil {
				t.Fatalf("Unexpected error: %s", err)
			}
			activeSpan := opentracing.SpanFromContext(ctx)
			if activeSpan == nil {
				t.Fatalf("'activeSpan' is 'nil'")
			}
			if name := tracing.AsSpan(activeSpan).Name(); name != "redis.pipeline" {
				t.Fatalf("Incorrect span name %q", name)
			}

			if err = hooks.AfterProcessPipeline(ctx, cmds); err != nil {
				t.Fatalf("Unexpected error: %s", err)
			}
		},
	)
}
