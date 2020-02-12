package redisbp_test

import (
	"context"
	"testing"

	"github.com/go-redis/redis/v7"

	"github.com/reddit/baseplate.go/redisbp"
	"github.com/reddit/baseplate.go/tracing"
)

func TestSpanHook(t *testing.T) {
	ctx, _ := tracing.StartSpanFromThriftContext(context.Background(), "foo")
	hooks := redisbp.SpanHook{ClientName: "redis"}
	cmd := redis.NewStatusCmd("ping")

	t.Run(
		"Before/AfterProcess",
		func(t *testing.T) {
			ctx, err := hooks.BeforeProcess(ctx, cmd)
			if err != nil {
				t.Fatalf("Unexpected error: %s", err)
			}
			activeSpan := tracing.GetActiveSpan(ctx)
			if activeSpan == nil {
				t.Fatalf("'activeSpan' is 'nil'")
			}
			if activeSpan.Name() != "redis.ping" {
				t.Fatalf("Incorrect span name %q", activeSpan.Name())
			}

			if err = hooks.AfterProcess(ctx, cmd); err != nil {
				t.Fatalf("Unexpected error: %s", err)
			}
		},
	)

	t.Run(
		"Before/AfterProcessPipeline",
		func(t *testing.T) {
			cmds := []redis.Cmder{cmd}
			ctx, err := hooks.BeforeProcessPipeline(ctx, cmds)
			if err != nil {
				t.Fatalf("Unexpected error: %s", err)
			}
			activeSpan := tracing.GetActiveSpan(ctx)
			if activeSpan == nil {
				t.Fatalf("'activeSpan' is 'nil'")
			}
			if activeSpan.Name() != "redis.pipeline" {
				t.Fatalf("Incorrect span name %q", activeSpan.Name())
			}

			if err = hooks.AfterProcessPipeline(ctx, cmds); err != nil {
				t.Fatalf("Unexpected error: %s", err)
			}
		},
	)
}
