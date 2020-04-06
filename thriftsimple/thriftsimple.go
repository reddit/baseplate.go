package thriftsimple

import (
	"context"
	"errors"
	"io/ioutil"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/getsentry/raven-go"
	"github.com/reddit/baseplate.go/edgecontext"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/metricsbp"
	"github.com/reddit/baseplate.go/secrets"
	"github.com/reddit/baseplate.go/thriftbp"
	"github.com/reddit/baseplate.go/tracing"
	"gopkg.in/yaml.v2"
)

type BaseplateServerConfig struct {
	Addr string

	Timeout time.Duration

	Debug bool

	Metrics struct {
		Namespace string
		Endpoint  string
	}

	Secrets struct {
		Path string
	}

	Sentry struct {
		DSN         string
		Environment string
		SampleRate  float64
	}

	Tracing struct {
		Namespace     string
		Endpoint      string
		RecordTimeout time.Duration `yaml:"recordTimeout"`
		SampleRate    float64
	}
}

func (c BaseplateServerConfig) AsConfig() BaseplateServerConfig {
	return c
}

func ParseBaseplateServerConfig(path string, cfg interface{}) error {
	if path == "" {
		return errors.New("no config path given")
	}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	if err = yaml.Unmarshal(data, &cfg); err != nil {
		return err
	}

	log.Debugf("%#v", cfg)
	return nil
}
func initLogger(debug bool) (log.Level, log.Wrapper) {
	var logLevel log.Level
	if debug {
		logLevel = log.DebugLevel
	} else {
		logLevel = log.WarnLevel
	}
	log.InitLogger(logLevel)
	return logLevel, log.ZapWrapper(logLevel)
}

func initSecrets(ctx context.Context, cfg BaseplateServerConfig, logger log.Wrapper) (*secrets.Store, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	secretsStore, err := secrets.NewStore(ctx, cfg.Secrets.Path, logger)
	if err != nil {
		return nil, err
	}
	return secretsStore, nil
}

func initTracing(cfg BaseplateServerConfig, logger log.Wrapper, metrics *metricsbp.Statsd) error {
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

func initSentry(cfg BaseplateServerConfig) error {
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
func NewBaseplateThriftServer(cfg BaseplateServerConfig, processor thriftbp.BaseplateProcessor, additionalMiddlewares ...thriftbp.Middleware) (*thrift.TSimpleServer, error) {
	ctx, _ := context.WithCancel(context.Background())

	logLevel, logger := initLogger(cfg.Debug)

	metricsbp.M = metricsbp.NewStatsd(ctx, metricsbp.StatsdConfig{
		Prefix:   cfg.Metrics.Namespace,
		Address:  cfg.Metrics.Endpoint,
		LogLevel: logLevel,
	})

	secretsStore, err := initSecrets(ctx, cfg, logger)
	if err != nil {
		log.Fatal(err)
	}
	defer secretsStore.Close()

	ecImpl := edgecontext.Init(edgecontext.Config{Store: secretsStore})
	if err = initTracing(cfg, logger, metricsbp.M); err != nil {
		log.Fatal(err)
	}
	if err = initSentry(cfg); err != nil {
		log.Fatal(err)
	}
	innerCfg := thriftbp.ServerConfig{
		Addr:    cfg.Addr,
		Timeout: cfg.Timeout,
		Logger:  logger,
	}

	middlewares := []thriftbp.Middleware{
		edgecontext.InjectThriftEdgeContext(ecImpl, logger),
		tracing.InjectThriftServerSpan,
	}
	middlewares = append(middlewares, additionalMiddlewares...)

	return thriftbp.NewThriftServer(innerCfg, processor, middlewares...)
}
