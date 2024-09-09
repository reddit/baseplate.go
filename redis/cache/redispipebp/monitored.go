package redispipebp

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/joomcode/errorx"
	"github.com/joomcode/redispipe/redis"
	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/reddit/baseplate.go/internal/prometheusbpint"
	"github.com/reddit/baseplate.go/prometheusbp"
	"github.com/reddit/baseplate.go/redis/cache/redisx"
	"github.com/reddit/baseplate.go/redis/internal/redisprom"
	"github.com/reddit/baseplate.go/tracing"
)

const (
	promNamespace = "redispipebp"

	labelSlug    = "redis_slug"
	labelCommand = "redis_command"
	labelSuccess = "redis_success"
)

var (
	labels = []string{
		labelSlug,
		labelCommand,
		labelSuccess,
	}

	promHistogram = promauto.With(prometheusbpint.GlobalRegistry).NewHistogramVec(prometheus.HistogramOpts{
		Namespace: promNamespace,
		Name:      "latency_seconds",
		Help:      "Redis latency in seconds",
		Buckets:   prometheusbp.DefaultLatencyBuckets,
	}, labels)
)

// MonitoredSync wraps Sync methods in client spans.
type MonitoredSync struct {
	Sync    redisx.Sync
	Name    string
	Cluster string
}

func extractCommand(cmd string) string {
	if index := strings.IndexByte(cmd, ' '); index >= 0 {
		return cmd[:index]
	}
	return cmd
}

// Do wraps s.Sync.Do in a client span.
func (s MonitoredSync) Do(ctx context.Context, cmd string, args ...interface{}) (result interface{}) {
	span, ctx := opentracing.StartSpanFromContext(
		ctx,
		s.Name+".do",
		tracing.SpanTypeOption{Type: tracing.SpanTypeClient},
	)
	command := extractCommand(cmd)
	active := redisprom.ActiveRequests.With(prometheus.Labels{
		redisprom.ClientNameLabel: s.Name,
		redisprom.CommandLabel:    command,
		redisprom.DatabaseLabel:   "", // We don't have that info
		redisprom.TypeLabel:       "", // We don't have that info
		redisprom.DeploymentLabel: "", // We don't have that info
		redisprom.ClusterLabel:    s.Cluster,
	})
	active.Inc()
	defer func(start time.Time) {
		active.Dec()
		err := redis.AsError(result)
		durationSeconds := time.Since(start).Seconds()
		promHistogram.With(prometheus.Labels{
			labelSlug:    s.Name,
			labelCommand: command,
			labelSuccess: prometheusbp.BoolString(err == nil),
		}).Observe(durationSeconds)
		redisprom.LatencySeconds.With(prometheus.Labels{
			redisprom.ClientNameLabel: s.Name,
			redisprom.CommandLabel:    command,
			redisprom.SuccessLabel:    prometheusbp.BoolString(err == nil),
			redisprom.DatabaseLabel:   "", // We don't have that info
			redisprom.TypeLabel:       "", // We don't have that info
			redisprom.DeploymentLabel: "", // We don't have that info
			redisprom.ClusterLabel:    s.Cluster,
		}).Observe(durationSeconds)
		redisprom.RequestsTotal.With(prometheus.Labels{
			redisprom.ClientNameLabel: s.Name,
			redisprom.CommandLabel:    command,
			redisprom.SuccessLabel:    prometheusbp.BoolString(err == nil),
			redisprom.DatabaseLabel:   "", // We don't have that info
			redisprom.TypeLabel:       "", // We don't have that info
			redisprom.DeploymentLabel: "", // We don't have that info
			redisprom.ClusterLabel:    s.Cluster,
		}).Inc()
		span.FinishWithOptions(tracing.FinishOptions{
			Ctx: ctx,
			Err: err,
		}.Convert())
	}(time.Now())

	return s.Sync.Do(ctx, cmd, args...)
}

// Send wraps s.Sync.Send in a client span.
func (s MonitoredSync) Send(ctx context.Context, r redis.Request) (result interface{}) {
	span, ctx := opentracing.StartSpanFromContext(
		ctx,
		s.Name+".send",
		tracing.SpanTypeOption{Type: tracing.SpanTypeClient},
	)
	command := extractCommand(r.Cmd)
	if command == "" {
		command = "send"
	}
	active := redisprom.ActiveRequests.With(prometheus.Labels{
		redisprom.ClientNameLabel: s.Name,
		redisprom.CommandLabel:    command,
		redisprom.DatabaseLabel:   "", // We don't have that info
		redisprom.TypeLabel:       "", // We don't have that info
		redisprom.DeploymentLabel: "", // We don't have that info
		redisprom.ClusterLabel:    s.Cluster,
	})
	active.Inc()
	defer func(start time.Time) {
		active.Dec()
		err := redis.AsError(result)
		durationSeconds := time.Since(start).Seconds()
		promHistogram.With(prometheus.Labels{
			labelSlug:    s.Name,
			labelCommand: command,
			labelSuccess: prometheusbp.BoolString(err == nil),
		}).Observe(durationSeconds)
		redisprom.LatencySeconds.With(prometheus.Labels{
			redisprom.ClientNameLabel: s.Name,
			redisprom.CommandLabel:    command,
			redisprom.SuccessLabel:    prometheusbp.BoolString(err == nil),
			redisprom.DatabaseLabel:   "", // We don't have that info
			redisprom.TypeLabel:       "", // We don't have that info
			redisprom.DeploymentLabel: "", // We don't have that info
			redisprom.ClusterLabel:    s.Cluster,
		}).Observe(durationSeconds)
		redisprom.RequestsTotal.With(prometheus.Labels{
			redisprom.ClientNameLabel: s.Name,
			redisprom.CommandLabel:    command,
			redisprom.SuccessLabel:    prometheusbp.BoolString(err == nil),
			redisprom.DatabaseLabel:   "", // We don't have that info
			redisprom.TypeLabel:       "", // We don't have that info
			redisprom.DeploymentLabel: "", // We don't have that info
			redisprom.ClusterLabel:    s.Cluster,
		}).Inc()
		span.FinishWithOptions(tracing.FinishOptions{
			Ctx: ctx,
			Err: redis.AsError(result),
		}.Convert())
	}(time.Now())

	return s.Sync.Send(ctx, r)
}

// SendMany wraps s.Sync.SendMany in a client span.
func (s MonitoredSync) SendMany(ctx context.Context, reqs []redis.Request) (results []interface{}) {
	const command = "send-many"
	span, ctx := opentracing.StartSpanFromContext(
		ctx,
		s.Name+"."+command,
		tracing.SpanTypeOption{Type: tracing.SpanTypeClient},
	)
	active := redisprom.ActiveRequests.With(prometheus.Labels{
		redisprom.ClientNameLabel: s.Name,
		redisprom.CommandLabel:    command,
		redisprom.DatabaseLabel:   "", // We don't have that info
		redisprom.TypeLabel:       "", // We don't have that info
		redisprom.DeploymentLabel: "", // We don't have that info
		redisprom.ClusterLabel:    s.Cluster,
	})
	active.Inc()
	defer func(start time.Time) {
		active.Dec()
		var err error
		if len(results) > 0 {
			first := redis.AsError(results[0])
			// We don't want to send an "error" to the span unless the request "failed"
			// which, if you have a single redis.ErrRequestCancelled result, all
			// of them will be that.
			var canceled *errorx.Error
			var wrapped *RedispipeError
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				err = first
			} else if errors.As(first, &canceled) {
				if canceled.IsOfType(redis.ErrRequestCancelled) {
					err = canceled
				}
			} else if errors.As(first, &wrapped) {
				if wrapped.Errorx.IsOfType(redis.ErrRequestCancelled) {
					err = wrapped
				}
			}
		}
		durationSeconds := time.Since(start).Seconds()
		promHistogram.With(prometheus.Labels{
			labelSlug:    s.Name,
			labelCommand: command,
			labelSuccess: prometheusbp.BoolString(err == nil),
		}).Observe(durationSeconds)
		redisprom.LatencySeconds.With(prometheus.Labels{
			redisprom.ClientNameLabel: s.Name,
			redisprom.CommandLabel:    command,
			redisprom.SuccessLabel:    prometheusbp.BoolString(err == nil),
			redisprom.DatabaseLabel:   "", // We don't have that info
			redisprom.TypeLabel:       "", // We don't have that info
			redisprom.DeploymentLabel: "", // We don't have that info
			redisprom.ClusterLabel:    s.Cluster,
		}).Observe(durationSeconds)
		redisprom.RequestsTotal.With(prometheus.Labels{
			redisprom.ClientNameLabel: s.Name,
			redisprom.CommandLabel:    command,
			redisprom.SuccessLabel:    prometheusbp.BoolString(err == nil),
			redisprom.DatabaseLabel:   "", // We don't have that info
			redisprom.TypeLabel:       "", // We don't have that info
			redisprom.DeploymentLabel: "", // We don't have that info
			redisprom.ClusterLabel:    s.Cluster,
		}).Inc()
		span.FinishWithOptions(tracing.FinishOptions{
			Ctx: ctx,
			Err: err,
		}.Convert())
	}(time.Now())

	return s.Sync.SendMany(ctx, reqs)
}

// SendTransaction wraps s.Sync.SendTransaction in a client span.
func (s MonitoredSync) SendTransaction(ctx context.Context, reqs []redis.Request) (results []interface{}, err error) {
	const command = "send-transaction"
	span, ctx := opentracing.StartSpanFromContext(
		ctx,
		s.Name+"."+command,
		tracing.SpanTypeOption{Type: tracing.SpanTypeClient},
	)
	active := redisprom.ActiveRequests.With(prometheus.Labels{
		redisprom.ClientNameLabel: s.Name,
		redisprom.CommandLabel:    command,
		redisprom.DatabaseLabel:   "", // We don't have that info
		redisprom.TypeLabel:       "", // We don't have that info
		redisprom.DeploymentLabel: "", // We don't have that info
		redisprom.ClusterLabel:    s.Cluster,
	})
	active.Inc()
	defer func(start time.Time) {
		active.Dec()
		durationSeconds := time.Since(start).Seconds()
		promHistogram.With(prometheus.Labels{
			labelSlug:    s.Name,
			labelCommand: command,
			labelSuccess: prometheusbp.BoolString(err == nil),
		}).Observe(durationSeconds)
		redisprom.LatencySeconds.With(prometheus.Labels{
			redisprom.ClientNameLabel: s.Name,
			redisprom.CommandLabel:    command,
			redisprom.SuccessLabel:    prometheusbp.BoolString(err == nil),
			redisprom.DatabaseLabel:   "", // We don't have that info
			redisprom.TypeLabel:       "", // We don't have that info
			redisprom.DeploymentLabel: "", // We don't have that info
			redisprom.ClusterLabel:    s.Cluster,
		}).Observe(durationSeconds)
		redisprom.RequestsTotal.With(prometheus.Labels{
			redisprom.ClientNameLabel: s.Name,
			redisprom.CommandLabel:    command,
			redisprom.SuccessLabel:    prometheusbp.BoolString(err == nil),
			redisprom.DatabaseLabel:   "", // We don't have that info
			redisprom.TypeLabel:       "", // We don't have that info
			redisprom.DeploymentLabel: "", // We don't have that info
			redisprom.ClusterLabel:    s.Cluster,
		}).Inc()
		span.FinishWithOptions(tracing.FinishOptions{
			Ctx: ctx,
			Err: err,
		}.Convert())
	}(time.Now())

	return s.Sync.SendTransaction(ctx, reqs)
}

// Scanner returns a new MonitoredScanIterator.
func (s MonitoredSync) Scanner(ctx context.Context, opts redis.ScanOpts) redisx.ScanIterator {
	return MonitoredScanIterator{
		ScanIterator: s.Sync.Scanner(ctx, opts),
		name:         s.Name,
		cluster:      s.Cluster,
		ctx:          ctx,
	}
}

// MonitoredScanIterator is a ScanIterator that is wrapped with a client spans.
type MonitoredScanIterator struct {
	redisx.ScanIterator

	name    string
	cluster string
	ctx     context.Context
}

// Next wraps s.ScanIterator.Next in a client span.
func (s MonitoredScanIterator) Next() (results []string, err error) {
	span, ctx := opentracing.StartSpanFromContext(
		s.ctx,
		s.name+".scanner.next",
		tracing.SpanTypeOption{Type: tracing.SpanTypeClient},
	)
	const command = "scanner-next"
	active := redisprom.ActiveRequests.With(prometheus.Labels{
		redisprom.ClientNameLabel: s.name,
		redisprom.CommandLabel:    command,
		redisprom.DatabaseLabel:   "", // We don't have that info
		redisprom.TypeLabel:       "", // We don't have that info
		redisprom.DeploymentLabel: "", // We don't have that info
		redisprom.ClusterLabel:    s.cluster,
	})
	active.Inc()
	defer func(start time.Time) {
		active.Dec()
		durationSeconds := time.Since(start).Seconds()
		promHistogram.With(prometheus.Labels{
			labelSlug:    s.name,
			labelCommand: command,
			labelSuccess: prometheusbp.BoolString(err == nil),
		}).Observe(durationSeconds)
		redisprom.LatencySeconds.With(prometheus.Labels{
			redisprom.ClientNameLabel: s.name,
			redisprom.CommandLabel:    command,
			redisprom.SuccessLabel:    prometheusbp.BoolString(err == nil),
			redisprom.DatabaseLabel:   "", // We don't have that info
			redisprom.TypeLabel:       "", // We don't have that info
			redisprom.DeploymentLabel: "", // We don't have that info
			redisprom.ClusterLabel:    s.cluster,
		}).Observe(durationSeconds)
		redisprom.RequestsTotal.With(prometheus.Labels{
			redisprom.ClientNameLabel: s.name,
			redisprom.CommandLabel:    command,
			redisprom.SuccessLabel:    prometheusbp.BoolString(err == nil),
			redisprom.DatabaseLabel:   "", // We don't have that info
			redisprom.TypeLabel:       "", // We don't have that info
			redisprom.DeploymentLabel: "", // We don't have that info
			redisprom.ClusterLabel:    s.cluster,
		}).Inc()
		span.FinishWithOptions(tracing.FinishOptions{
			Ctx: ctx,
			Err: err,
		}.Convert())
	}(time.Now())

	return s.ScanIterator.Next()
}

var (
	_ redisx.Sync = MonitoredSync{}
)
