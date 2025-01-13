package redisbp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"

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

func (h SpanHook) DialHook(hook redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := hook(ctx, network, addr)
		return conn, err
	}
}

func (h SpanHook) ProcessHook(hook redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		ctx = h.startChildSpan(ctx, cmd.Name())
		err := hook(ctx, cmd)
		h.endChildSpan(ctx, err)
		return err
	}
}

func (h SpanHook) ProcessPipelineHook(hook redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		ctx = h.startChildSpan(ctx, "pipeline")
		err := hook(ctx, cmds)
		errs := make([]error, 0, len(cmds))
		for _, cmd := range cmds {
			if err := cmd.Err(); !errors.Is(err, redis.Nil) {
				errs = append(errs, err)
			}
		}
		if err != nil {
			errs = append(errs, err)
		}

		h.endChildSpan(ctx, errors.Join(errs...))
		return nil
	}
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
