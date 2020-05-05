package baseplate

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	yaml "gopkg.in/yaml.v2"

	"github.com/reddit/baseplate.go/batcherror"
	"github.com/reddit/baseplate.go/edgecontext"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/metricsbp"
	"github.com/reddit/baseplate.go/runtimebp"
	"github.com/reddit/baseplate.go/secrets"
	"github.com/reddit/baseplate.go/tracing"
)

// Config is a general purpose config for assembling a Baseplate server
type Config struct {
	// Addr is the local address to run your server on.
	//
	// It should be in the format "${IP}:${Port}", "localhost:${Port}",
	// or simply ":${Port}".
	Addr string `yaml:"addr"`

	// Timeout is the socket connection timeout for Servers that
	// support that.
	Timeout time.Duration `yaml:"timeout"`

	// StopTimeout is the timeout for the Stop command for the service.
	//
	// If this is not set, then no timeout will be set on the Stop command.
	StopTimeout time.Duration `yaml:"stopTimeout"`

	Log     log.Config       `yaml:"log"`
	Metrics metricsbp.Config `yaml:"metrics"`
	Runtime runtimebp.Config `yaml:"runtime"`
	Secrets secrets.Config   `yaml:"secrets"`
	Sentry  log.SentryConfig `yaml:"setry"`
	Tracing tracing.Config   `yaml:"tracing"`
}

// Baseplate is the general purpose object that you build a Server on.
type Baseplate interface {
	io.Closer

	Config() Config
	EdgeContextImpl() *edgecontext.Impl
	Secrets() *secrets.Store
}

// Server is the primary interface for baseplate servers.
type Server interface {
	// Close should stop the server gracefully and only return after the server has
	// finished shutting down.
	//
	// It is recommended that you use baseplate.Serve() rather than calling Close
	// directly as baseplate.Serve will manage starting your service as well as
	// shutting it down gracefully in response to a shutdown signal.
	io.Closer

	// Baseplate returns the Baseplate object the server is built on.
	Baseplate() Baseplate

	// Serve should start the Server on the Addr given by the Config and only
	// return once the Server has stopped.
	//
	// It is recommended that you use baseplate.Serve() rather than calling Serve
	// directly as baseplate.Serve will manage starting your service as well as
	// shutting it down gracefully.
	Serve() error
}

// Serve runs the given Server until it is given an external shutdown signal
// using runtimebp.HandleShutdown to handle the signal and shut down the
// server gracefully.  Returns the (possibly nil) error returned by "Close" or
// context.DeadlineExceeded if it times out.
//
// If a StopTimeout is configure, Serve will wait for that duration for the
// server to stop before timing out and returning to force a shutdown.
//
// This is the recommended way to run a Baseplate Server rather than calling
// server.Start/Stop directly.
func Serve(ctx context.Context, server Server) error {
	shutdownChannel := make(chan error)
	go runtimebp.HandleShutdown(
		ctx,
		func(signal os.Signal) {
			timeout := server.Baseplate().Config().StopTimeout
			ctx := context.Background()
			if timeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, timeout)
				defer cancel()
			}

			var err error
			closeChannel := make(chan error)
			go func() {
				closeChannel <- server.Close()
			}()

			// Wait for either the context to timeout or Stop() to finish.
			select {
			case <-ctx.Done():
				err = ctx.Err()
			case e := <-closeChannel:
				err = e
			}

			log.Infow(
				"graceful shutdown",
				"signal", signal,
				"close error", err,
			)
			shutdownChannel <- err
		},
	)
	log.Info(server.Serve())
	return <-shutdownChannel
}

// ParseConfig returns a new Config parsed from the YAML file at the given path.
func ParseConfig(path string, serviceCfg interface{}) (Config, error) {
	cfg := Config{}
	if path == "" {
		return cfg, errors.New("baseplate.ParseConfig: no config path given")
	}

	f, err := os.Open(path)
	if err != nil {
		return cfg, err
	}
	defer f.Close()

	return DecodeConfigYAML(f, serviceCfg)
}

// DecodeConfigYAML returns a new Config built from decoding the YAML read from
// the given Reader.
func DecodeConfigYAML(reader io.ReadSeeker, serviceCfg interface{}) (Config, error) {
	cfg := Config{}
	if err := yaml.NewDecoder(reader).Decode(&cfg); err != nil {
		return cfg, err
	}

	log.Debugf("%#v", cfg)

	if serviceCfg != nil {
		reader.Seek(0, io.SeekStart)
		if err := yaml.NewDecoder(reader).Decode(serviceCfg); err != nil {
			return cfg, err
		}
	}
	return cfg, nil
}

type cancelCloser struct {
	cancel context.CancelFunc
}

func (c cancelCloser) Close() error {
	c.cancel()
	return nil
}

// New parses the config file at the given path, initializes the monitoring and
// logging frameworks, and returns the "serve" context and a new Baseplate to
// run your service on.
//
// serviceCfg is optional, if it is non-nil, it should be a pointer and New
// will also decode the config file at the path to set it up.  This can be used
// to parse additional, service specific config values from the same config
// file.
func New(ctx context.Context, path string, serviceCfg interface{}) (Baseplate, error) {
	cfg, err := ParseConfig(path, serviceCfg)
	if err != nil {
		return nil, err
	}
	bp := impl{cfg: cfg}

	runtimebp.InitFromConfig(cfg.Runtime)

	ctx, cancel := context.WithCancel(ctx)
	bp.closers = append(bp.closers, cancelCloser{cancel})

	log.InitFromConfig(cfg.Log)
	bp.closers = append(bp.closers, metricsbp.InitFromConfig(ctx, cfg.Metrics))

	closer, err := log.InitSentry(cfg.Sentry)
	if err != nil {
		bp.Close()
		return nil, err
	}
	bp.closers = append(bp.closers, closer)

	bp.secrets, err = secrets.InitFromConfig(ctx, cfg.Secrets)
	if err != nil {
		bp.Close()
		return nil, err
	}
	bp.closers = append(bp.closers, bp.secrets)

	closer, err = tracing.InitFromConfig(cfg.Tracing)
	if err != nil {
		bp.Close()
		return nil, err
	}
	bp.closers = append(bp.closers, closer)

	bp.ecImpl = edgecontext.Init(edgecontext.Config{
		Store:  bp.secrets,
		Logger: log.ErrorWithSentryWrapper(),
	})
	return bp, nil
}

type impl struct {
	cfg     Config
	closers []io.Closer
	ecImpl  *edgecontext.Impl
	secrets *secrets.Store
}

func (bp impl) Config() Config {
	return bp.cfg
}

func (bp impl) Secrets() *secrets.Store {
	return bp.secrets
}

func (bp impl) EdgeContextImpl() *edgecontext.Impl {
	return bp.ecImpl
}

func (bp impl) Close() error {
	batch := &batcherror.BatchError{}
	for _, c := range bp.closers {
		if err := c.Close(); err != nil {
			log.Errorw(
				"Failed to close closer",
				"err", err,
				"closer", fmt.Sprintf("%#v", c),
			)
			batch.Add(err)
		}
	}
	return batch.Compile()
}

// NewTestBaseplate returns a new Baseplate using the given Config and secrets
// Store that can be used in testing.
//
// NewTestBaseplate only returns a Baseplate, it does not initialialize any of
// the monitoring or logging frameworks.
func NewTestBaseplate(cfg Config, store *secrets.Store) Baseplate {
	return &impl{
		cfg:     cfg,
		secrets: store,
		ecImpl:  edgecontext.Init(edgecontext.Config{Store: store}),
	}
}

var (
	_ Baseplate = impl{}
	_ Baseplate = (*impl)(nil)
)
