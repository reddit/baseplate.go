package integrations_test

import (
	"context"
	"testing"

	"github.com/go-redis/redis/v7"

	"github.com/reddit/baseplate.go/integrations"
	"github.com/reddit/baseplate.go/tracing"
)

func TestRedisSpanHook(t *testing.T) {
	ctx := context.Background()
	span := tracing.StartSpanFromThriftContext(ctx, "foo")
	ctx, _ = span.SetServerSpan(ctx)
	hooks := integrations.RedisSpanHook{ClientName: "redis"}
	cmd := redis.NewStatusCmd("ping")

	t.Run(
		"Before/AfterProcess",
		func(t *testing.T) {
			ctx, err := hooks.BeforeProcess(ctx, cmd)
			if err != nil {
				t.Fatalf("Unexpected error: %s", err)
			}
			childSpan := tracing.GetChildSpan(ctx)
			if childSpan == nil {
				t.Fatalf("'childSpan' is 'nil'")
			}
			if childSpan.Name != "redis.ping" {
				t.Fatalf("Incorrect span name '%s'", childSpan.Name)
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
			childSpan := tracing.GetChildSpan(ctx)
			if childSpan == nil {
				t.Fatalf("'childSpan' is 'nil'")
			}
			if childSpan.Name != "redis.pipeline" {
				t.Fatalf("Incorrect span name '%s'", childSpan.Name)
			}

			if err = hooks.AfterProcessPipeline(ctx, cmds); err != nil {
				t.Fatalf("Unexpected error: %s", err)
			}
		},
	)
}
