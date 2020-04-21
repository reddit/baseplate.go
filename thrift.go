package baseplate

import (
	"context"
	"io"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/getsentry/raven-go"
	"github.com/opentracing/opentracing-go"

	"github.com/reddit/baseplate.go/batcherror"
	"github.com/reddit/baseplate.go/edgecontext"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/metricsbp"
	"github.com/reddit/baseplate.go/secrets"
	"github.com/reddit/baseplate.go/thriftbp"
	"github.com/reddit/baseplate.go/tracing"
)

type baseplateThriftServer struct {
	thriftServer *thrift.TSimpleServer
	config       ServerConfig
	closers      []io.Closer
	logger       log.Wrapper
	secretsStore *secrets.Store
}

func (bts *baseplateThriftServer) Config() ServerConfig {
	return bts.config
}
func (bts *baseplateThriftServer) Secrets() *secrets.Store {
	return bts.secretsStore
}

func (bts *baseplateThriftServer) Serve() error {
	return bts.thriftServer.Serve()
}

func (bts *baseplateThriftServer) Close() error {
	var errors batcherror.BatchError
	errors.Add(bts.thriftServer.Stop())
	for _, c := range bts.closers {
		errors.Add(c.Close())
	}
	return errors.Compile()
}

func initLogger(cfg ServerConfig) log.Wrapper {
	if cfg.Log.Level == "" {
		cfg.Log.Level = log.InfoLevel
	}
	level := cfg.Log.Level
	log.InitLogger(level)
	return log.ZapWrapper(level)
}

func initSecrets(ctx context.Context, cfg ServerConfig, logger log.Wrapper) (*secrets.Store, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	secretsStore, err := secrets.NewStore(ctx, cfg.Secrets.Path, logger)
	if err != nil {
		return nil, err
	}
	return secretsStore, nil
}

func initTracing(cfg ServerConfig, logger log.Wrapper, metrics *metricsbp.Statsd) error {
	if err := tracing.InitGlobalTracer(tracing.TracerConfig{
		ServiceName:      cfg.Tracing.Namespace,
		SampleRate:       cfg.Tracing.SampleRate,
		Logger:           logger,
		MaxRecordTimeout: cfg.Tracing.RecordTimeout,
		QueueName:        "main",
	}); err != nil {
		return err
	}

	tracing.RegisterCreateServerSpanHooks(
		metricsbp.CreateServerSpanHook{Metrics: metrics},
		tracing.ErrorReporterCreateServerSpanHook{},
	)
	return nil
}

func initSentry(cfg ServerConfig) error {
	if err := raven.SetDSN(cfg.Sentry.DSN); err != nil {
		return err
	}
	if err := raven.SetSampleRate(float32(cfg.Sentry.SampleRate)); err != nil {
		return err
	}
	raven.SetEnvironment(cfg.Sentry.Environment)
	return nil
}

func cleanup(closers []io.Closer) {
	for _, c := range closers {
		if err := c.Close(); err != nil {
			log.Errorw("msg", "err", err)
		}
	}
}

// NewThriftServer returns a server that initializes and includes the default
// middleware and cross-cutting concerns needed in order to be a standard Baseplate service.
//
// At the moment, this includes secrets management, metrics, edge contexts
// (edgecontext.InjectThriftEdgeContext), and spans/tracing (tracing.InjectThriftServerSpan).
func NewThriftServer(ctx context.Context, cfg ServerConfig, processor thriftbp.BaseplateProcessor, additionalMiddlewares ...thriftbp.ProcessorMiddleware) (srv Server, err error) {
	var closers []io.Closer
	defer func() {
		if err != nil {
			cleanup(closers)
		}
	}()

	logger := initLogger(cfg)

	metricsbp.M = metricsbp.NewStatsd(ctx, metricsbp.StatsdConfig{
		CounterSampleRate:   &cfg.Metrics.CounterSampleRate,
		HistogramSampleRate: &cfg.Metrics.HistogramSampleRate,
		Prefix:              cfg.Metrics.Namespace,
		Address:             cfg.Metrics.Endpoint,
		LogLevel:            cfg.Log.Level,
	})
	closers = append(closers, metricsbp.M)

	secretsStore, err := initSecrets(ctx, cfg, logger)
	if err != nil {
		return nil, err
	}
	closers = append(closers, secretsStore)

	ecImpl := edgecontext.Init(edgecontext.Config{Store: secretsStore})
	if err = initTracing(cfg, logger, metricsbp.M); err != nil {
		return nil, err
	}
	closers = append(closers, opentracing.GlobalTracer().(*tracing.Tracer))
	if err = initSentry(cfg); err != nil {
		return nil, err
	}
	innerCfg := thriftbp.ServerConfig{
		Addr:    cfg.Addr,
		Timeout: cfg.Timeout,
		Logger:  logger,
	}

	middlewares := []thriftbp.ProcessorMiddleware{
		thriftbp.InjectServerSpan,
		thriftbp.InjectEdgeContext(ecImpl),
	}
	middlewares = append(middlewares, additionalMiddlewares...)

	ts, err := thriftbp.NewServer(innerCfg, processor, middlewares...)
	if err != nil {
		return nil, err
	}
	srv = &baseplateThriftServer{
		logger:       logger,
		config:       cfg,
		closers:      closers,
		thriftServer: ts,
		secretsStore: secretsStore,
	}
	return
}
