package redispipebp

import (
	"context"
	"errors"

	"github.com/joomcode/errorx"
	"github.com/joomcode/redispipe/redis"
	"github.com/opentracing/opentracing-go"

	"github.com/reddit/baseplate.go/redis/redisx"
	"github.com/reddit/baseplate.go/tracing"
)

// MonitoredSync wraps Sync methods in client spans.
type MonitoredSync struct {
	Sync redisx.Sync
	Name string
}

// Do wraps s.Sync.Do in a client span.
func (s MonitoredSync) Do(ctx context.Context, cmd string, args ...interface{}) (result interface{}) {
	span, ctx := opentracing.StartSpanFromContext(
		ctx,
		s.Name+".do",
		tracing.SpanTypeOption{Type: tracing.SpanTypeClient},
	)
	defer func() {
		span.FinishWithOptions(tracing.FinishOptions{
			Ctx: ctx,
			Err: redis.AsError(result),
		}.Convert())
	}()

	return s.Sync.Do(ctx, cmd, args...)
}

// Send wraps s.Sync.Send in a client span.
func (s MonitoredSync) Send(ctx context.Context, r redis.Request) (result interface{}) {
	span, ctx := opentracing.StartSpanFromContext(
		ctx,
		s.Name+".send",
		tracing.SpanTypeOption{Type: tracing.SpanTypeClient},
	)
	defer func() {
		span.FinishWithOptions(tracing.FinishOptions{
			Ctx: ctx,
			Err: redis.AsError(result),
		}.Convert())
	}()

	return s.Sync.Send(ctx, r)
}

// SendMany wraps s.Sync.SendMany in a client span.
func (s MonitoredSync) SendMany(ctx context.Context, reqs []redis.Request) (results []interface{}) {
	span, ctx := opentracing.StartSpanFromContext(
		ctx,
		s.Name+".send-many",
		tracing.SpanTypeOption{Type: tracing.SpanTypeClient},
	)
	defer func() {
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
		span.FinishWithOptions(tracing.FinishOptions{
			Ctx: ctx,
			Err: err,
		}.Convert())
	}()

	return s.Sync.SendMany(ctx, reqs)
}

// SendTransaction wraps s.Sync.SendTransaction in a client span.
func (s MonitoredSync) SendTransaction(ctx context.Context, reqs []redis.Request) (results []interface{}, err error) {
	span, ctx := opentracing.StartSpanFromContext(
		ctx,
		s.Name+".send-transaction",
		tracing.SpanTypeOption{Type: tracing.SpanTypeClient},
	)
	defer func() {
		span.FinishWithOptions(tracing.FinishOptions{
			Ctx: ctx,
			Err: err,
		}.Convert())
	}()

	return s.Sync.SendTransaction(ctx, reqs)
}

// Scanner returns a new MonitoredScanIterator.
func (s MonitoredSync) Scanner(ctx context.Context, opts redis.ScanOpts) redisx.ScanIterator {
	return MonitoredScanIterator{
		ScanIterator: s.Sync.Scanner(ctx, opts),
		name:         s.Name + ".scanner",
		ctx:          ctx,
	}
}

// MonitoredScanIterator is a ScanIterator that is wrapped with a client spans.
type MonitoredScanIterator struct {
	redisx.ScanIterator

	name string
	ctx  context.Context
}

// Next wraps s.ScanIterator.Next in a client span.
func (s MonitoredScanIterator) Next() (results []string, err error) {
	span, ctx := opentracing.StartSpanFromContext(
		s.ctx,
		s.name+".next",
		tracing.SpanTypeOption{Type: tracing.SpanTypeClient},
	)
	defer func() {
		span.FinishWithOptions(tracing.FinishOptions{
			Ctx: ctx,
			Err: err,
		}.Convert())
	}()
	return s.ScanIterator.Next()
}

var (
	_ redisx.Sync = MonitoredSync{}
)
