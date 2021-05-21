package baseplate

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	yaml "gopkg.in/yaml.v2"

	"github.com/reddit/baseplate.go/batchcloser"
	"github.com/reddit/baseplate.go/ecinterface"
	"github.com/reddit/baseplate.go/errorsbp"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/metricsbp"
	"github.com/reddit/baseplate.go/runtimebp"
	"github.com/reddit/baseplate.go/secrets"
	"github.com/reddit/baseplate.go/tracing"
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

	// Timeout is the socket connection timeout for Servers that
	// support that.
	Timeout time.Duration `yaml:"timeout"`

	// StopTimeout is the timeout for the Stop command for the service.
	//
	// If this is not set, then a default value of 30 seconds will be used.
	// If this is less than 0, then no timeout will be set on the Stop command.
	StopTimeout time.Duration `yaml:"stopTimeout"`

	Log     log.Config       `yaml:"log"`
	Metrics metricsbp.Config `yaml:"metrics"`
	Runtime runtimebp.Config `yaml:"runtime"`
	Secrets secrets.Config   `yaml:"secrets"`
	Sentry  log.SentryConfig `yaml:"sentry"`
	Tracing tracing.Config   `yaml:"tracing"`
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

			// Initialize a channel to pass the result of server.Close().
			//
			// It's buffered with size 1 to avoid blocking the goroutine forever.
			closeChannel := make(chan error, 1)

			// Tell the server and any provided closers to close.
			//
			// This is a blocking call, so it is called in a separate goroutine.
			go func() {
				bc := batchcloser.New(args.PreShutdown...)
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

// parseConfig parses from the YAML file at the given path into cfg.
func parseConfig(path string, cfg Configer) error {
	if path == "" {
		return errors.New("baseplate.ParseConfig: no config path given")
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return DecodeConfigYAML(f, cfg)
}

// DecodeConfigYAML decods the YAML read from the given Reader into cfg.
//
// It enables strict parsing,
// if there are yaml tags not defined in cfg passed in,
// that will cause an error.
//
// If you don't have any customized configurations to decode from YAML,
// you can just pass in a *pointer* to baseplate.Config:
//
//     var cfg baseplate.Config
//     baseplate.DecodeConfigYAML(reader, &cfg)
//
// If you do have customized configurations to decode from YAML,
// the recommended way of passing the strict yaml parsing is to embed
// baseplate.Config with `yaml:",inline"` yaml tags, for example:
//
//     type myServiceConfig struct {
//       // The yaml tag is required to pass strict parsing.
//       baseplate.Config `yaml:",inline"`
//
//       // Actual configs
//       FancyName string `yaml:"fancy_name"`
//     }
//     var cfg myServiceCfg
//     baseplate.DecodeConfigYAML(reader, &cfg)
func DecodeConfigYAML(reader io.Reader, cfg Configer) error {
	decoder := yaml.NewDecoder(reader)
	decoder.SetStrict(true)
	return decoder.Decode(cfg)
}

// NewArgs defines the args used in New functino.
type NewArgs struct {
	// Required.
	ConfigPath string

	// Required. New will panic if this is not set.
	//
	// The factory to be used to create edge context implementation.
	EdgeContextFactory ecinterface.Factory

	// Optional.
	//
	// If ServiceCfg is non-nil,
	// it should usually be a pointer to a struct with baseplate.Config embedded,
	// with `yaml:",inline"` yaml tags. For example:
	//
	//     type myServiceConfig struct {
	//       // The yaml tag is required to pass strict parsing.
	//       baseplate.Config `yaml:",inline"`
	//
	//       // Actual configs
	//       FancyName string `yaml:"fancy_name"`
	//     }
	//     var cfg myServiceCfg
	//     baseplate.New(ctx, baseplate.NewArgs{
	//       ServiceCfg: &cfg,
	//       // other args
	//     })
	//
	// Please refer to DecodeConfigYAML's doc for more details.
	ServiceCfg Configer
}

// New parses the config file at the given path, initializes the monitoring and
// logging frameworks, and returns the "serve" context and a new Baseplate to
// run your service on.  The returned context will be cancelled when the
// Baseplate is closed.
func New(ctx context.Context, args NewArgs) (context.Context, Baseplate, error) {
	var cfger Configer
	if args.ServiceCfg != nil {
		cfger = args.ServiceCfg
	} else {
		cfger = new(Config)
	}
	err := parseConfig(args.ConfigPath, cfger)
	if err != nil {
		return nil, nil, err
	}
	cfg := cfger.GetConfig()
	bp := impl{cfg: cfg, closers: batchcloser.New()}

	runtimebp.InitFromConfig(cfg.Runtime)

	ctx, cancel := context.WithCancel(ctx)
	bp.closers.Add(batchcloser.WrapCancel(cancel))

	log.InitFromConfig(cfg.Log)
	bp.closers.Add(metricsbp.InitFromConfig(ctx, cfg.Metrics))

	closer, err := log.InitSentry(cfg.Sentry)
	if err != nil {
		bp.Close()
		return nil, nil, err
	}
	bp.closers.Add(closer)

	bp.secrets, err = secrets.InitFromConfig(ctx, cfg.Secrets)
	if err != nil {
		bp.Close()
		return nil, nil, err
	}
	bp.closers.Add(bp.secrets)

	closer, err = tracing.InitFromConfig(cfg.Tracing)
	if err != nil {
		bp.Close()
		return nil, nil, err
	}
	bp.closers.Add(closer)

	bp.ecImpl, err = args.EdgeContextFactory(ecinterface.FactoryArgs{
		Store: bp.secrets,
	})
	if err != nil {
		bp.Close()
		return nil, nil, err
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

	var errs []error
	var batchErr errorsbp.Batch
	if errors.As(err, &batchErr) {
		errs = batchErr.GetErrors()
	} else if err != nil {
		errs = append(errs, err)
	}

	for _, batchedErr := range errs {
		var closeErr batchcloser.CloseError
		if errors.As(batchedErr, &closeErr) {
			log.Errorw(
				"Failed to close closer",
				"err", closeErr.Unwrap(),
				"closer", closeErr.Closer,
			)
		} else {
			log.Errorw(
				"Error while closing unknown closer",
				"err", batchedErr,
			)
		}
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
