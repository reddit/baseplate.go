package baseplate

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"time"

	"github.com/reddit/baseplate.go/batchcloser"
	"github.com/reddit/baseplate.go/configbp"
	"github.com/reddit/baseplate.go/ecinterface"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/metricsbp"
	"github.com/reddit/baseplate.go/prometheusbp"
	"github.com/reddit/baseplate.go/runtimebp"
	"github.com/reddit/baseplate.go/secrets"
	"github.com/reddit/baseplate.go/tracing"
)

const (
	// DefaultStopDelay is the default StopDelay to be used in Serve.
	DefaultStopDelay = 5 * time.Second
)

// Configer defines the interface that allows you to extend Config with your own
// configurations.
type Configer interface {
	GetConfig() Config
}

var (
	_ Configer = Config{}
)

// Config is a general purpose config for assembling a Baseplate server.
//
// It implements Configer.
type Config struct {
	// Addr is the local address to run your server on.
	//
	// It should be in the format "${IP}:${Port}", "localhost:${Port}",
	// or simply ":${Port}".
	Addr string `yaml:"addr"`

	// Deprecated: No-op for now, will be removed in a future release.
	Timeout time.Duration `yaml:"timeout"`

	// StopTimeout is the timeout for the Stop command for the service.
	//
	// If this is not set, then a default value of 30 seconds will be used.
	// If this is less than 0, then no timeout will be set on the Stop command.
	StopTimeout time.Duration `yaml:"stopTimeout"`

	// Delay after receiving termination signal (SIGTERM, etc.) before kicking off
	// the graceful shutdown process. This happens before the PreShutdown closers.
	//
	// By default this is 1s (DefaultStopDelay).
	// To disable it, set it to a negative value.
	StopDelay time.Duration `yaml:"stopDelay"`

	Log     log.Config       `yaml:"log"`
	Runtime runtimebp.Config `yaml:"runtime"`
	Secrets secrets.Config   `yaml:"secrets"`
	Sentry  log.SentryConfig `yaml:"sentry"`
	Tracing tracing.Config   `yaml:"tracing"`

	// Deprecated: statsd metrics are deprecated.
	Metrics metricsbp.Config `yaml:"metrics"`
}

// GetConfig implements Configer.
func (c Config) GetConfig() Config {
	return c
}

// Baseplate is the general purpose object that you build a Server on.
type Baseplate interface {
	io.Closer

	Configer

	EdgeContextImpl() ecinterface.Interface
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

// ServeArgs provides a list of arguments to Serve.
type ServeArgs struct {
	// Server is the Server that should be run until receiving a shutdown signal.
	// This is a required argument and baseplate.Serve will panic if this is nil.
	Server Server

	// PreShutdown is an optional slice of io.Closers that should be gracefully
	// shut down before the server upon receipt of a shutdown signal.
	PreShutdown []io.Closer

	// PostShutdown is an optional slice of io.Closers that should be gracefully
	// shut down after the server upon receipt of a shutdown signal.
	PostShutdown []io.Closer
}

// Serve runs the given Server until it is given an external shutdown signal.
//
// It uses runtimebp.HandleShutdown to handle the signal and gracefully shut
// down, in order:
//
// * any provided PreShutdown closers,
//
// * the Server, and
//
// * any provided PostShutdown closers.
//
// Returns the (possibly nil) error returned by "Close", or
// context.DeadlineExceeded if it times out.
//
// If a StopTimeout is configured, Serve will wait for that duration for the
// server to stop before timing out and returning to force a shutdown.
//
// This is the recommended way to run a Baseplate Server rather than calling
// server.Start/Stop directly.
func Serve(ctx context.Context, args ServeArgs) error {
	server := args.Server

	// Initialize a channel to return the response from server.Close() as our
	// return value.
	shutdownChannel := make(chan error)

	// Listen for a shutdown command.
	//
	// This is a blocking call so it is in a separate goroutine.  It will exit
	// either when it is triggered via a shutdown command or if the context passed
	// in is cancelled.
	go runtimebp.HandleShutdown(
		ctx,
		func(signal os.Signal) {
			// Check if the server has a StopTimeout configured.
			//
			// If one is set, we will only wait for that duration for the server to
			// stop gracefully and will exit after the deadline is exceeded.
			timeout := server.Baseplate().GetConfig().StopTimeout

			// Default to 30 seconds if not set.
			if timeout == 0 {
				timeout = time.Second * 30
			}

			// If timeout is < 0, we will wait indefinitely for the server to close.
			if timeout > 0 {
				// Declare cancel in advance so we can just use `=` when calling
				// context.WithTimeout.  If we used `:=` it would bind `ctx` to the scope
				// of this `if` statement rather than updating the value declared before.
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, timeout)
				defer cancel()
			}

			delay := server.Baseplate().GetConfig().StopDelay
			if delay == 0 {
				delay = DefaultStopDelay
			}

			// Initialize a channel to pass the result of server.Close().
			//
			// It's buffered with size 1 to avoid blocking the goroutine forever.
			closeChannel := make(chan error, 1)

			// Tell the server and any provided closers to close.
			//
			// This is a blocking call, so it is called in a separate goroutine.
			go func() {
				var bc batchcloser.BatchCloser
				if delay > 0 {
					bc.Add(batchcloser.Wrap(func() error {
						time.Sleep(delay)
						return nil
					}))
				}
				bc.Add(args.PreShutdown...)
				bc.Add(server)
				bc.Add(args.PostShutdown...)
				closeChannel <- bc.Close()
			}()

			// Declare the error variable we will use later here so we can set it to
			// the result of the switch statement below.
			var err error

			// Wait for either ctx.Done() to be closed (indicating that the context
			// was cancelled or its deadline was exceeded) or server.Close() to return.
			select {
			case <-ctx.Done():
				// The context timed-out or was cancelled so use that error.
				err = fmt.Errorf("baseplate: context cancelled while waiting for server.Close(). %w", ctx.Err())
			case e := <-closeChannel:
				// server.Close() completed and passed its result to closeChannel, so
				// use that value.
				err = e
			}

			log.Infow(
				"graceful shutdown",
				"signal", signal,
				"close error", err,
			)

			// Pass the final, potentially nil, error to shutdownChannel.
			shutdownChannel <- err
		},
	)

	// Start the server.
	//
	// This is a blocking command and will run until the server is closed.
	log.Info(server.Serve())

	// Return the error passed via shutdownChannel to the caller.
	//
	// This will block until a value is put on the channel.
	return <-shutdownChannel
}

// ParseConfigYAML loads the baseplate config into the structed pointed to by cfgPointer.
//
// The configuration file is located based on the $BASEPLATE_CONFIG_PATH
// environment variable.
//
// To avoid easy mistakes, "strict" mode is enabled while parsing the file,
// which means that all values in the YAML config must have a matching
// struct or value during decoding.
//
// If you don't have any customized configurations to decode from YAML,
// you can just pass in a *pointer* to baseplate.Config:
//
//	var cfg baseplate.Config
//	if err := baseplate.ParseConfigYAML(&cfg); err != nil {
//	  log.Fatalf("Parsing config: %s", err)
//	}
//	ctx, bp, err := baseplate.New(baseplate.NewArgs{
//	  EdgeContextFactory: edgecontext.Factory(...),
//	  Config:             cfg,
//	})
//
// If you do have customized configurations to decode from YAML,
// embed a baseplate.Config with `yaml:",inline"` yaml tags, for example:
//
//	type myServiceConfig struct {
//	  // The yaml tag is required to pass strict parsing.
//	  baseplate.Config `yaml:",inline"`
//
//	  // Actual configs
//	  FancyName string `yaml:"fancy_name"`
//	}
//	var cfg myServiceCfg
//	if err := baseplate.ParseConfigYAML(&cfg); err != nil {
//	  log.Fatalf("Parsing config: %s", err)
//	}
//	ctx, bp, err := baseplate.New(baseplate.NewArgs{
//	  EdgeContextFactory: edgecontext.Factory(...),
//	  Config:             cfg,
//	})
//
// Environment variable references (e.g. $FOO and ${FOO}) are substituted into the
// YAML from the process-level environment before parsing the configuration.
func ParseConfigYAML(cfgPointer Configer) error {
	if configbp.BaseplateConfigPath == "" {
		return fmt.Errorf("no $BASEPLATE_CONFIG_PATH specified, cannot load config")
	}
	return configbp.ParseStrictFile(configbp.BaseplateConfigPath, cfgPointer)
}

// NewArgs defines the args used in New functino.
type NewArgs struct {
	// Required. New will panic if this is nil.
	Config Configer

	// Required. New will panic if this is nil.
	//
	// The factory to be used to create edge context implementation.
	EdgeContextFactory ecinterface.Factory
}

// New initializes Baseplate libraries with the given config,
// (logging, secrets, tracing, edge context, etc.),
// and returns the "serve" context and a new Baseplate to
// run your service on.
// The returned context will be cancelled when the Baseplate is closed.
func New(ctx context.Context, args NewArgs) (context.Context, Baseplate, error) {
	cfg := args.Config.GetConfig()
	bp := impl{cfg: cfg, closers: batchcloser.New()}

	if info, ok := debug.ReadBuildInfo(); ok {
		prometheusbp.RecordModuleVersions(info)
	} else {
		log.C(ctx).Warn("baseplate.New: unable to read build info to export dependency metrics")
	}

	runtimebp.InitFromConfig(cfg.Runtime)

	ctx, cancel := context.WithCancel(ctx)
	bp.closers.Add(batchcloser.WrapCancel(cancel))

	log.InitFromConfig(cfg.Log)

	closer, err := log.InitSentry(cfg.Sentry)
	if err != nil {
		bp.Close()
		return nil, nil, fmt.Errorf(
			"baseplate.New: failed to init sentry: %w (config: %#v)",
			err,
			cfg.Sentry,
		)
	}
	bp.closers.Add(closer)

	bp.secrets, err = secrets.InitFromConfig(ctx, cfg.Secrets)
	if err != nil {
		bp.Close()
		return nil, nil, fmt.Errorf(
			"baseplate.New: failed to init secrets: %w (config: %#v)",
			err,
			cfg.Secrets,
		)
	}
	bp.closers.Add(bp.secrets)

	closer, err = tracing.InitFromConfig(cfg.Tracing)
	if err != nil {
		bp.Close()
		return nil, nil, fmt.Errorf(
			"baseplate.New: failed to init tracing: %w (config: %#v)",
			err,
			cfg.Tracing,
		)
	}
	bp.closers.Add(closer)

	bp.ecImpl, err = args.EdgeContextFactory(ecinterface.FactoryArgs{
		Store: bp.secrets,
	})
	if err != nil {
		bp.Close()
		return nil, nil, fmt.Errorf(
			"baseplate.New: failed to init edge context: %w",
			err,
		)
	}

	return ctx, bp, nil
}

type impl struct {
	closers *batchcloser.BatchCloser
	cfg     Config
	ecImpl  ecinterface.Interface
	secrets *secrets.Store
}

func (bp impl) GetConfig() Config {
	return bp.cfg
}

func (bp impl) Secrets() *secrets.Store {
	return bp.secrets
}

func (bp impl) EdgeContextImpl() ecinterface.Interface {
	return bp.ecImpl
}

func (bp impl) Close() error {
	err := bp.closers.Close()
	if err != nil {
		log.Errorw(
			"Error while closing closers",
			"err", err,
		)
	}
	return err
}

// NewTestBaseplateArgs defines the args used by NewTestBaseplate.
type NewTestBaseplateArgs struct {
	Config          Config
	Store           *secrets.Store
	EdgeContextImpl ecinterface.Interface
}

// NewTestBaseplate returns a new Baseplate using the given Config and secrets
// Store that can be used in testing.
//
// NewTestBaseplate only returns a Baseplate, it does not initialize any of
// the monitoring or logging frameworks.
func NewTestBaseplate(args NewTestBaseplateArgs) Baseplate {
	return &impl{
		cfg:     args.Config,
		secrets: args.Store,
		ecImpl:  args.EdgeContextImpl,
		closers: batchcloser.New(),
	}
}

var (
	_ Baseplate = impl{}
	_ Baseplate = (*impl)(nil)
)
