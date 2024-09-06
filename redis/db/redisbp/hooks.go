package redisbp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/reddit/baseplate.go/internal/prometheusbpint"
	"github.com/reddit/baseplate.go/prometheusbp"
	"github.com/reddit/baseplate.go/redis/internal/redisprom"
	"github.com/reddit/baseplate.go/tracing"
)

type promCtxKeyType struct{}

var promCtxKey promCtxKeyType

type promCtx struct {
	command string
	start   time.Time
}

// SpanHook is a redis.Hook for wrapping Redis commands and pipelines
// in Client Spans and metrics.
type SpanHook struct {
	ClientName string
	Type       string
	Deployment string
	Database   string
	// The cluster identifier based on the connection address. If we cannot identify
	// a cluster based on connection address this field will be empty.
	Cluster string

	promActive *prometheusbpint.HighWatermarkGauge
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
	errs := make([]error, 0, len(cmds))
	for _, cmd := range cmds {
		if err := cmd.Err(); !errors.Is(err, redis.Nil) {
			errs = append(errs, err)
		}
	}
	h.endChildSpan(ctx, errors.Join(errs...))
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
	if h.promActive != nil {
		h.promActive.Inc()
	}
	redisprom.ActiveRequests.With(prometheus.Labels{
		redisprom.ClientNameLabel: h.ClientName,
		redisprom.TypeLabel:       h.Type,
		redisprom.CommandLabel:    cmdName,
		redisprom.DeploymentLabel: h.Deployment,
		redisprom.DatabaseLabel:   h.Database,
		redisprom.ClusterLabel:    h.Cluster,
	}).Inc()
	return context.WithValue(ctx, promCtxKey, &promCtx{
		command: cmdName,
		start:   time.Now(),
	})
}

func (h SpanHook) endChildSpan(ctx context.Context, err error) {
	command := "unknown"
	if v, _ := ctx.Value(promCtxKey).(*promCtx); v != nil {
		command = v.command
		durationSeconds := time.Since(v.start).Seconds()
		latencyTimer.With(prometheus.Labels{
			nameLabel:    h.ClientName,
			commandLabel: v.command,
			successLabel: prometheusbp.BoolString(err == nil),
		}).Observe(durationSeconds)
		redisprom.LatencySeconds.With(prometheus.Labels{
			redisprom.ClientNameLabel: h.ClientName,
			redisprom.TypeLabel:       h.Type,
			redisprom.CommandLabel:    command,
			redisprom.DeploymentLabel: h.Deployment,
			redisprom.SuccessLabel:    prometheusbp.BoolString(err == nil),
			redisprom.DatabaseLabel:   h.Database,
			redisprom.ClusterLabel:    h.Cluster,
		}).Observe(durationSeconds)
	}
	// Outside of the context casting because we always want this to work.
	redisprom.RequestsTotal.With(prometheus.Labels{
		redisprom.ClientNameLabel: h.ClientName,
		redisprom.TypeLabel:       h.Type,
		redisprom.CommandLabel:    command,
		redisprom.DeploymentLabel: h.Deployment,
		redisprom.SuccessLabel:    prometheusbp.BoolString(err == nil),
		redisprom.DatabaseLabel:   h.Database,
		redisprom.ClusterLabel:    h.Cluster,
	}).Inc()
	redisprom.ActiveRequests.With(prometheus.Labels{
		redisprom.ClientNameLabel: h.ClientName,
		redisprom.TypeLabel:       h.Type,
		redisprom.CommandLabel:    command,
		redisprom.DeploymentLabel: h.Deployment,
		redisprom.DatabaseLabel:   h.Database,
		redisprom.ClusterLabel:    h.Cluster,
	}).Dec()
	if h.promActive != nil {
		h.promActive.Dec()
	}

	if span := opentracing.SpanFromContext(ctx); span != nil {
		span.FinishWithOptions(tracing.FinishOptions{
			Ctx: ctx,
			Err: err,
		}.Convert())
	}
}
