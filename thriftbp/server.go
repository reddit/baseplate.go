package thriftbp

import (
	"errors"
	"fmt"
	"time"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/reddit/baseplate.go"
	"github.com/reddit/baseplate.go/errorsbp"
	"github.com/reddit/baseplate.go/log"
)

var (
	errInvalidListenerAddress = errors.New("invalid listener address")
)

// ServerConfig is the arg struct for both NewServer and NewBaseplateServer.
//
// Some of the fields are only used by NewServer and some of them are only used
// by NewBaseplateServer. Please refer to the documentation for each field to
// see how is it used.
type ServerConfig struct {
	// Required, used by both NewServer and NewBaseplateServer.
	//
	// This is the thrift processor implementation to handle endpoints.
	Processor thrift.TProcessor

	// Optional, used by both NewServer and NewBaseplateServer.
	//
	// For NewServer, this defines all the middlewares to wrap the server with.
	// For NewBaseplateServer, this only defines the middlewares in addition to
	// (and after) BaseplateDefaultProcessorMiddlewares.
	Middlewares []thrift.ProcessorMiddleware

	// Optional, used only by NewServer.
	//
	// A log wrapper that is used by the TSimpleServer.
	Logger thrift.Logger

	// Optional, used only by NewBaseplateServer.
	//
	// Please refer to the documentation of
	// DefaultProcessorMiddlewaresArgs.ErrorSpanSuppressor for more details
	// regarding how it is used.
	ErrorSpanSuppressor errorsbp.Suppressor

	// Optional, used only by NewBaseplateServer.
	//
	// Report the payload size metrics with this sample rate.
	// If not set none of the requests will be sampled.
	ReportPayloadSizeMetricsSampleRate float64

	// Optional, used only by NewServer.
	// In NewBaseplateServer the address set in bp.Config() will be used instead.
	//
	// The endpoint address of your thrift service.
	//
	// This is ignored if Socket is non-nil.
	Addr string

	// Deprecated: No-op for now, will be removed in a future release.
	Timeout time.Duration

	// Optional, used only by NewServer.
	// In NewBaseplateServer the address and timeout set in bp.Config() will be
	// used instead.
	//
	// You can choose to set Socket instead of Addr.
	Socket *thrift.TServerSocket
}

// NewServer returns a thrift.TSimpleServer using the THeader transport
// and protocol to serve the given TProcessor which is wrapped with the
// given ProcessorMiddlewares.
func NewServer(cfg ServerConfig) (*thrift.TSimpleServer, error) {
	// Leaving this default socket initialization intact so that we
	// don't break existing usages of NewServer. Ideally, by the time
	// we've reached NewServer, all necessary configuration should
	// be completed.
	if cfg.Socket == nil {
		if err := withDefaultSocket()(&cfg); err != nil {
			return nil, err
		}
	}

	server := thrift.NewTSimpleServer4(
		thrift.WrapProcessor(cfg.Processor, cfg.Middlewares...),
		cfg.Socket,
		thrift.NewTHeaderTransportFactoryConf(nil, nil),
		thrift.NewTHeaderProtocolFactoryConf(nil),
	)
	server.SetForwardHeaders(HeadersToForward)
	server.SetLogger(cfg.Logger)
	return server, nil
}

// NewServerFromOptions creates a new ServerConfig instance and delegates
// to NewServer so that it can instantiate a new thrift.TSimpleServer.
// This is a low level api and should only be used at the recommendation
// of the baseplate maintainers. Most application implementations should
// rely on NewBaseplateServer.
func NewServerFromOptions(opts ...ServerOption) (*thrift.TSimpleServer, error) {
	cfg, err := buildConfig(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to build server config: %w", err)
	}
	return NewServer(cfg)
}

// NewBaseplateServer returns a new Thrift implementation of a Baseplate
// server with the given config. This is the primary constructor that
// should be used when building a baseplate server.
func NewBaseplateServer(
	bp baseplate.Baseplate,
	cfg ServerConfig,
) (baseplate.Server, error) {
	options := append(
		[]ServerOption{withConfigFrom(cfg)},
		DefaultServerOptions(bp)...,
	)
	return NewBaseplateServerFromOptions(bp, options...)
}

// NewBaseplateServerFromOptions returns a new Thrift implementation of a Baseplate
// server using a config built from the supplied opts ServerOption. This is a low
// level api and should  only be used at the recommendation of the baseplate maintainers.
// Most application implementations should rely on NewBaseplateServer.
func NewBaseplateServerFromOptions(
	bp baseplate.Baseplate,
	opts ...ServerOption,
) (baseplate.Server, error) {
	srv, err := NewServerFromOptions(opts...)
	if err != nil {
		return nil, err
	}
	return ApplyBaseplate(bp, srv), nil
}

// ServerOption is a type used for defining configuration arguments
// needed for creating a baseplate server. It allows returning
// an error to signal that a configuration value was invalid
// or failed to be set.
type ServerOption func(cfg *ServerConfig) error

// buildConfig creates a new ServerConfig instance and applies
// all the supplied configuration options to it.
func buildConfig(options ...ServerOption) (ServerConfig, error) {
	cfg := &ServerConfig{}
	for _, option := range options {
		err := option(cfg)
		if err != nil {
			return ServerConfig{}, err
		}
	}

	return *cfg, nil
}

// withConfigFrom returns a server option that overwrites the
// any existing values in a ServerConfig with those defined
// in src. This is meant to be used from NewBaseplateServer
// for mapping the supplied ServerConfig into a ServerOption
func withConfigFrom(src ServerConfig) ServerOption {
	return func(dst *ServerConfig) error {
		*dst = src
		return nil
	}
}

// DefaultServerOptions builds and returns a slice of the default
// baseplate server config options. This method is meant to support
// advanced use cases when building a custom baseplate server.
func DefaultServerOptions(bp baseplate.Baseplate) []ServerOption {
	return []ServerOption{
		withDefaultMiddleware(bp),
		withDefaultLogger(bp),
		withDefaultListenerAddress(bp),
		withDefaultSocket(),
	}
}

// withDefaultMiddleware will prepend the default middleware so
// we can maintain backwards compatibility with the old logic of
// NewBaseplateServer.
func withDefaultMiddleware(bp baseplate.Baseplate) ServerOption {
	return func(cfg *ServerConfig) error {
		existingMiddleware := cfg.Middlewares

		if err := WithDefaultMiddleware(bp)(cfg); err != nil {
			return err
		}

		// Prepend defaults to existing values
		cfg.Middlewares = append(cfg.Middlewares, existingMiddleware...)
		return nil
	}
}

// WithDefaultMiddleware will append the default baseplate middleware to the
// server config. When depending on specific features such as ErrorSpanSuppressor,
// the ServerOption configuring the setting needs to be invoked before this method.
func WithDefaultMiddleware(bp baseplate.Baseplate) ServerOption {
	return func(cfg *ServerConfig) error {
		return WithMiddleware(BaseplateDefaultProcessorMiddlewares(
			DefaultProcessorMiddlewaresArgs{
				EdgeContextImpl:                    bp.EdgeContextImpl(),
				ErrorSpanSuppressor:                cfg.ErrorSpanSuppressor,
				ReportPayloadSizeMetricsSampleRate: cfg.ReportPayloadSizeMetricsSampleRate,
			},
		)...)(cfg)
	}
}

func withDefaultLogger(bp baseplate.Baseplate) ServerOption {
	return WithDefaultLogger(bp.GetConfig().Log.Level)
}

// withDefaultListenerAddress is used to copy the address config from the
// baseplate config to the server config. Currently, it's only
// needed to guarantee backwards compatibility in NewServer
func withDefaultListenerAddress(bp baseplate.Baseplate) ServerOption {
	return WithListenerAddress(bp.GetConfig().Addr)
}

// WithMiddleware appends the arg middleware to the end of
// the cfg.Middleware slice
func WithMiddleware(middleware ...thrift.ProcessorMiddleware) ServerOption {
	return func(cfg *ServerConfig) error {
		cfg.Middlewares = append(cfg.Middlewares, middleware...)
		return nil
	}
}

// WithProcessor configures the TProcessor that will be used by
// the server to handle and respond to thrift api requests.
func WithProcessor(processor thrift.TProcessor) ServerOption {
	return func(cfg *ServerConfig) error {
		cfg.Processor = processor
		return nil
	}
}

// WithErrorSpanSuppressor configures the suppressor that will
// be used when constructing the InjectServerSpan middleware.
// This option must be set before any calls to WithDefaultMiddleware.
func WithErrorSpanSuppressor(suppressor errorsbp.Suppressor) ServerOption {
	return func(cfg *ServerConfig) error {
		cfg.ErrorSpanSuppressor = suppressor
		return nil
	}
}

// WithPayloadSizeMetricsSampleRate configures the sample rate used
// to report the payload size metrics for requests. This option must
// be set before any calls to WithDefaultMiddleware.
func WithPayloadSizeMetricsSampleRate(rate float64) ServerOption {
	return func(cfg *ServerConfig) error {
		cfg.ReportPayloadSizeMetricsSampleRate = rate
		return nil
	}
}

// WithLogger configures the thrift.Logger that will be used by the server
// to log connection errors.
func WithLogger(logger thrift.Logger) ServerOption {
	return func(cfg *ServerConfig) error {
		cfg.Logger = logger
		return nil
	}
}

// WithDefaultLogger initializes the logger that will be used by the thrift
// server to log connection errors. The logger will be configured with the
// level supplied.
func WithDefaultLogger(level log.Level) ServerOption {
	return WithLogger(
		log.ZapWrapper(
			log.ZapWrapperArgs{
				Level: level,
				KVPairs: map[string]interface{}{
					"from": "thrift",
				},
			},
		).ToThriftLogger(),
	)
}

// WithListenerAddress is defined to provide backwards compatibility with
// existing usages of NewServer. Use WithDefaultSocket instead.
func WithListenerAddress(address string) ServerOption {
	return func(cfg *ServerConfig) error {
		cfg.Addr = address
		return nil
	}
}

// WithSocket creates a server option that initializes a
// thrift.TServerSocket listening on the address supplied.
func WithSocket(socket *thrift.TServerSocket) ServerOption {
	return func(cfg *ServerConfig) error {
		cfg.Socket = socket
		return nil
	}
}

// withDefaultSocket initializes a socket listening at the
// configured address.
func withDefaultSocket() ServerOption {
	return func(cfg *ServerConfig) error {
		if cfg.Addr == "" {
			return fmt.Errorf("listener address is empty, %w", errInvalidListenerAddress)
		}

		socket, err := thrift.NewTServerSocket(cfg.Addr)
		if err != nil {
			return err
		}

		return WithSocket(socket)(cfg)
	}
}

// ApplyBaseplate returns the given TSimpleServer as a baseplate Server with the
// given Baseplate.
//
// You generally don't need to use this, instead use NewBaseplateServer, which
// will take care of this for you.
func ApplyBaseplate(bp baseplate.Baseplate, server *thrift.TSimpleServer) baseplate.Server {
	return impl{bp: bp, srv: server}
}

type impl struct {
	bp  baseplate.Baseplate
	srv *thrift.TSimpleServer
}

func (s impl) Baseplate() baseplate.Baseplate {
	return s.bp
}

func (s impl) Serve() error {
	return s.srv.Serve()
}

func (s impl) Close() error {
	return s.srv.Stop()
}

var (
	_ baseplate.Server = impl{}
	_ baseplate.Server = (*impl)(nil)
)
