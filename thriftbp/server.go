package thriftbp

import (
	"time"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/reddit/baseplate.go"
	"github.com/reddit/baseplate.go/errorsbp"
	"github.com/reddit/baseplate.go/log"
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
	var transport thrift.TServerTransport
	if cfg.Socket == nil {
		var err error
		transport, err = thrift.NewTServerSocket(cfg.Addr)
		if err != nil {
			return nil, err
		}
	} else {
		transport = cfg.Socket
	}

	server := thrift.NewTSimpleServer4(
		thrift.WrapProcessor(cfg.Processor, cfg.Middlewares...),
		transport,
		thrift.NewTHeaderTransportFactoryConf(nil, nil),
		thrift.NewTHeaderProtocolFactoryConf(nil),
	)
	server.SetForwardHeaders(HeadersToForward)
	server.SetLogger(cfg.Logger)
	return server, nil
}

// NewBaseplateServer returns a new Thrift implementation of a Baseplate
// server with the given config.
func NewBaseplateServer(
	bp baseplate.Baseplate,
	cfg ServerConfig,
) (baseplate.Server, error) {

	opts := []ServerOpt{
		func(optCfg ServerConfig) ServerConfig {
			return cfg
		},
	}
	opts = append(opts, DefaultServerOpts(bp)...)

	return NewBaseplateServerFromOpts(bp, opts...)
}

// NewBaseplateServerFromOpts returns a new Thrift implementation of a Baseplate
// server using a config built from the supplied opts ServerOpt.
func NewBaseplateServerFromOpts(
	bp baseplate.Baseplate,
	opts ...ServerOpt,
) (baseplate.Server, error) {

	srv, err := NewServer(BuildConfig(opts...))
	if err != nil {
		return nil, err
	}
	return ApplyBaseplate(bp, srv), nil
}

type ServerOpt func(cfg ServerConfig) ServerConfig

func BuildConfig(opts ...ServerOpt) ServerConfig {
	cfg := ServerConfig{}
	for _, opt := range opts {
		cfg = opt(cfg)
	}

	return cfg
}

func DefaultServerOpts(bp baseplate.Baseplate) []ServerOpt {
	return []ServerOpt{
		serverOptMiddleware(bp),
		serverOptLogger(bp),
		serverOptAddress(bp),
		serverOptSocket(bp),
	}
}

func serverOptMiddleware(bp baseplate.Baseplate) ServerOpt {
	return func(cfg ServerConfig) ServerConfig {
		middlewares := BaseplateDefaultProcessorMiddlewares(
			DefaultProcessorMiddlewaresArgs{
				EdgeContextImpl:                    bp.EdgeContextImpl(),
				ErrorSpanSuppressor:                cfg.ErrorSpanSuppressor,
				ReportPayloadSizeMetricsSampleRate: cfg.ReportPayloadSizeMetricsSampleRate,
			},
		)
		cfg.Middlewares = append(middlewares, cfg.Middlewares...)

		return cfg
	}
}

func serverOptLogger(bp baseplate.Baseplate) ServerOpt {
	return func(cfg ServerConfig) ServerConfig {
		cfg.Logger = log.ZapWrapper(log.ZapWrapperArgs{
			Level: bp.GetConfig().Log.Level,
			KVPairs: map[string]interface{}{
				"from": "thrift",
			},
		}).ToThriftLogger()

		return cfg
	}
}

func serverOptAddress(bp baseplate.Baseplate) ServerOpt {
	return func(cfg ServerConfig) ServerConfig {
		cfg.Addr = bp.GetConfig().Addr

		return cfg
	}
}

func serverOptSocket(bp baseplate.Baseplate) ServerOpt {
	return func(cfg ServerConfig) ServerConfig {
		cfg.Socket = nil
		return cfg
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
