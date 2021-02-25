package sqlbp

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"time"

	"github.com/go-kit/kit/metrics"
	"github.com/jmoiron/sqlx"
	"github.com/luna-duclos/instrumentedsql"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/reddit/baseplate.go/metricsbp"
	"github.com/reddit/baseplate.go/secrets"
	"github.com/reddit/baseplate.go/tracing"
)

type tracer struct {
	prefix string
}

type span struct {
	tracer
	parent opentracing.Span
	err    error
}

// NewTracer returns a tracer that will fetch spans using opentracing's SpanFromContext function.
func NewTracer(prefix string) instrumentedsql.Tracer {
	return tracer{
		prefix: prefix,
	}
}

// GetSpan returns a span from the context, or creates one.
func (t tracer) GetSpan(ctx context.Context) instrumentedsql.Span {

	if ctx == nil {
		parent := opentracing.StartSpan(
			t.prefix,
			tracing.SpanTypeOption{
				Type: tracing.SpanTypeClient,
			})

		return &span{
			parent: parent,
			tracer: t,
		}
	}

	return &span{parent: opentracing.SpanFromContext(ctx), tracer: t}
}

func (s *span) NewChild(name string) instrumentedsql.Span {
	name = s.tracer.prefix + name
	// s.parent == nil when db.query is used instead of db.queryContext
	if s.parent == nil {
		return &span{parent: opentracing.StartSpan(name), tracer: s.tracer}
	}

	return &span{
		parent: s.parent.Tracer().StartSpan(
			name,
			opentracing.ChildOf(s.parent.Context()),
			tracing.SpanTypeOption{
				Type: tracing.SpanTypeClient,
			},
		),
		tracer: s.tracer,
	}
}

func (s *span) SetLabel(k, v string) {
	s.parent.SetTag(k, v)
}

func (s *span) SetError(err error) {
	if err == nil || err == driver.ErrSkip {
		return
	}
	s.err = err
}

func (s *span) Finish() {
	s.parent.FinishWithOptions(tracing.FinishOptions{
		Err: s.err,
	}.Convert())
}

type connPoolMetrics struct {
	// Pool Status
	Open  metrics.Gauge
	Idle  metrics.Gauge
	InUse metrics.Gauge
	// Counters
	WaitCount         metrics.Counter // The total number of connections waited for.
	MaxIdleClosed     metrics.Counter // The total number of connections closed due to SetMaxIdleConns.
	MaxIdleTimeClosed metrics.Counter // The total number of connections closed due to SetMaxIdleTime.
	MaxLifetimeClosed metrics.Counter // The total number of connections closed due to SetConnMaxLifetime.
	// Timing data
	WaitDuration metrics.Histogram // The total time blocked waiting for a new connection.
}

// Begin a goroutine which sets the pool metrics.
func collectPoolMetrics(
	ctx context.Context,
	name string,
	tickerDuration time.Duration,
	db *sqlx.DB,
) {
	metrics := connPoolMetrics{
		Open:              metricsbp.M.RuntimeGauge(getConnPoolMetric(name, "open")),
		InUse:             metricsbp.M.RuntimeGauge(getConnPoolMetric(name, "in_use")),
		Idle:              metricsbp.M.RuntimeGauge(getConnPoolMetric(name, "idle")),
		WaitCount:         metricsbp.M.Counter(getConnPoolMetric(name, "wait_count")),
		MaxIdleClosed:     metricsbp.M.Counter(getConnPoolMetric(name, "max_idle_closed")),
		MaxIdleTimeClosed: metricsbp.M.Counter(getConnPoolMetric(name, "max_idle_time_closed")),
		MaxLifetimeClosed: metricsbp.M.Counter(getConnPoolMetric(name, "max_lifetime_closed")),
		WaitDuration:      metricsbp.M.Histogram(getConnPoolMetric(name, "wait_duration")),
	}
	if tickerDuration <= 0 {
		tickerDuration = 500 * time.Millisecond
	}
	ticker := time.NewTicker(tickerDuration)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			stats := db.Stats()
			metrics.Open.Set(float64(stats.OpenConnections))
			metrics.InUse.Set(float64(stats.InUse))
			metrics.Idle.Set(float64(stats.Idle))
			metrics.WaitCount.Add(float64(stats.WaitCount))
			metrics.MaxIdleClosed.Add(float64(stats.MaxIdleClosed))
			metrics.MaxIdleTimeClosed.Add(float64(stats.MaxIdleTimeClosed))
			metrics.MaxLifetimeClosed.Add(float64(stats.MaxLifetimeClosed))
			metrics.WaitDuration.Observe(float64(stats.WaitDuration.Milliseconds()))
		}
	}
}

func getConnPoolMetric(name string, metric string) string {
	return "clients." + name + ".pool." + metric
}

// Instantiate the pool and begin monitoring.
func InitMonitoredDBPool(ctx context.Context, s *secrets.Store, driverName string, driver driver.Driver, cfg Config) (*sqlx.DB, error) {
	name := WrapSQLDriver(cfg.Name, driverName, driver)

	connStr, err := cfg.GetConnectionString(s)
	if err != nil {
		return nil, err
	}

	db, err := sqlx.Open(name, connStr)
	if err != nil {
		return nil, err
	}

	// Max open should be equal to or greater than max idle.
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetMaxOpenConns(cfg.MaxOpenConns)

	// The maximum lifetime of a connection, regardless of whether it's idle.
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	if cfg.PoolMetricsInterval != 0 {
		go collectPoolMetrics(ctx, name, cfg.PoolMetricsInterval, db)
	}
	return db, nil
}

// Set up tracing and wrap driver specified in config.
func WrapSQLDriver(name, driverName string, driver driver.Driver) string {
	wrappedName := "instrumented-" + driverName + "-" + name
	for _, d := range sql.Drivers() {
		if d == wrappedName {
			return wrappedName
		}
	}
	// Tell sqlx to use the BindType for the original driver for your new, wrapped
	// driver.
	sqlx.BindDriver(wrappedName, sqlx.BindType(driverName))
	sql.Register(wrappedName, instrumentedsql.WrapDriver(
		driver,
		instrumentedsql.WithTracer(NewTracer(name+".")),
	))
	return wrappedName
}
