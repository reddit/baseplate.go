package baseplate

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/apache/thrift/lib/go/thrift"

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

func initLogger(cfg ServerConfig) {
	if cfg.Log.Level == "" {
		cfg.Log.Level = log.InfoLevel
	}
	level := cfg.Log.Level
	log.InitLogger(level)
}

func initSecrets(ctx context.Context, cfg ServerConfig) (*secrets.Store, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	secretsStore, err := secrets.NewStore(ctx, cfg.Secrets.Path, log.ErrorWithSentryWrapper())
	if err != nil {
		return nil, err
	}
	return secretsStore, nil
}

func initTracing(cfg ServerConfig) (io.Closer, error) {
	closer, err := tracing.InitGlobalTracerWithCloser(tracing.TracerConfig{
		ServiceName:      cfg.Tracing.Namespace,
		SampleRate:       cfg.Tracing.SampleRate,
		MaxRecordTimeout: cfg.Tracing.RecordTimeout,
		QueueName:        cfg.Tracing.QueueName,
		Logger:           log.ErrorWithSentryWrapper(),
	})
	if err != nil {
		return nil, err
	}

	tracing.RegisterCreateServerSpanHooks(
		metricsbp.CreateServerSpanHook{},
		tracing.ErrorReporterCreateServerSpanHook{},
	)
	return closer, nil
}

func initSentry(cfg ServerConfig) (io.Closer, error) {
	return log.InitSentry(log.SentryConfig{
		DSN:         cfg.Sentry.DSN,
		SampleRate:  cfg.Sentry.SampleRate,
		Environment: cfg.Sentry.Environment,
	})
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
			for _, c := range closers {
				if err := c.Close(); err != nil {
					log.Errorw(
						"Failed to close closer",
						"err", err,
						"closer", fmt.Sprintf("%#v", c),
					)
				}
			}
		}
	}()

	initLogger(cfg)

	closer, err := initSentry(cfg)
	if err != nil {
		return nil, err
	}
	closers = append(closers, closer)

	metricsbp.M = metricsbp.NewStatsd(ctx, metricsbp.StatsdConfig{
		CounterSampleRate:   cfg.Metrics.CounterSampleRate,
		HistogramSampleRate: cfg.Metrics.HistogramSampleRate,
		Prefix:              cfg.Metrics.Namespace,
		Address:             cfg.Metrics.Endpoint,
		LogLevel:            log.ErrorLevel,
	})
	closers = append(closers, metricsbp.M)

	secretsStore, err := initSecrets(ctx, cfg)
	if err != nil {
		return nil, err
	}
	closers = append(closers, secretsStore)

	ecImpl := edgecontext.Init(edgecontext.Config{Store: secretsStore})

	closer, err = initTracing(cfg)
	if err != nil {
		return nil, err
	}
	closers = append(closers, closer)

	innerCfg := thriftbp.ServerConfig{
		Addr:    cfg.Addr,
		Timeout: cfg.Timeout,
		Logger:  thrift.Logger(log.ZapWrapper(log.ErrorLevel)),
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
		config:       cfg,
		closers:      closers,
		thriftServer: ts,
		secretsStore: secretsStore,
	}
	return
}
