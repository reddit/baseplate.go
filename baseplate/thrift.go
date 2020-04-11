package baseplate

import (
	"context"
	"io"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/getsentry/raven-go"
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
	beforeStop   []io.Closer
	afterStop    []io.Closer
	logger       log.Wrapper
}

func (bts *baseplateThriftServer) Config() ServerConfig {
	return bts.config
}

func (bts *baseplateThriftServer) Impl() interface{} {
	return bts.thriftServer
}

func (bts *baseplateThriftServer) Serve() error {
	return bts.thriftServer.Serve()
}

func (bts *baseplateThriftServer) Stop() error {
	var err error = nil
	for _, c := range bts.beforeStop {
		c.Close()
	}
	bts.thriftServer.Stop()
	for _, c := range bts.afterStop {
		c.Close()
	}
	return err
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

// NewBaseplateThriftServer returns a server that includes the default middleware.
func NewBaseplateThriftServer(ctx context.Context, cfg ServerConfig, processor thriftbp.BaseplateProcessor, additionalMiddlewares ...thriftbp.Middleware) (Server, error) {
	beforeStop := make([]io.Closer, 0)
	afterStop := make([]io.Closer, 0)
	logger := initLogger(cfg)

	metricsbp.M = metricsbp.NewStatsd(ctx, metricsbp.StatsdConfig{
		Prefix:   cfg.Metrics.Namespace,
		Address:  cfg.Metrics.Endpoint,
		LogLevel: cfg.Log.Level,
	})

	secretsStore, err := initSecrets(ctx, cfg, logger)
	if err != nil {
		return nil, err
	}
	afterStop = append(afterStop, secretsStore)

	ecImpl := edgecontext.Init(edgecontext.Config{Store: secretsStore})
	if err = initTracing(cfg, logger, metricsbp.M); err != nil {
		return nil, err
	}
	if err = initSentry(cfg); err != nil {
		return nil, err
	}
	innerCfg := thriftbp.ServerConfig{
		Addr:    cfg.Addr,
		Timeout: cfg.Timeout,
		Logger:  logger,
	}

	middlewares := []thriftbp.Middleware{
		tracing.InjectThriftServerSpan,
		edgecontext.InjectThriftEdgeContext(ecImpl, logger),
	}
	middlewares = append(middlewares, additionalMiddlewares...)

	ts, err := thriftbp.NewThriftServer(innerCfg, processor, middlewares...)
	if err != nil {
		return nil, err
	}

	return &baseplateThriftServer{
		logger:       logger,
		config:       cfg,
		afterStop:    afterStop,
		beforeStop:   beforeStop,
		thriftServer: ts,
	}, nil
}