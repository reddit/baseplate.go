package thriftbp

import (
	"time"

	"github.com/apache/thrift/lib/go/thrift"

	baseplate "github.com/reddit/baseplate.go"
	"github.com/reddit/baseplate.go/log"
)

// ServerConfig is the arg struct for NewServer.
type ServerConfig struct {
	// The endpoint address of your thrift service.
	//
	// This is ignored if Socket is non-nil.
	Addr string

	// The timeout for the underlying thrift.TServerSocket transport.
	//
	// If your clients are using client pools,
	// you usually want this timeout to be long in order to keep clients alive.
	//
	// This is ignored if Socket is non-nil.
	Timeout time.Duration

	// A log wrapper that is used by the TSimpleServer.
	//
	// It's compatible with log.Wrapper (with an extra typecasting),
	// but you should not use log.ErrorWithSentryWrapper for this one,
	// as it would log all the network I/O errors,
	// which would be too spammy for sentry.
	Logger thrift.Logger

	// Optional TServerSocket you can use instead of setting one up using Addr
	// plus timeout.  If provided, this will be used rather than Addr and Timeout.
	Socket *thrift.TServerSocket
}

// NewServer returns a thrift.TSimpleServer using the THeader transport
// and protocol to serve the given TProcessor which is wrapped with the
// given ProcessorMiddlewares.
func NewServer(
	cfg ServerConfig,
	processor thrift.TProcessor,
	middlewares ...thrift.ProcessorMiddleware,
) (*thrift.TSimpleServer, error) {
	var transport *thrift.TServerSocket
	if cfg.Socket == nil {
		var err error
		transport, err = thrift.NewTServerSocketTimeout(cfg.Addr, cfg.Timeout)
		if err != nil {
			return nil, err
		}
	} else {
		transport = cfg.Socket
	}

	server := thrift.NewTSimpleServer4(
		thrift.WrapProcessor(processor, middlewares...),
		transport,
		thrift.NewTHeaderTransportFactory(nil),
		thrift.NewTHeaderProtocolFactory(),
	)
	server.SetForwardHeaders(HeadersToForward)
	server.SetLogger(cfg.Logger)
	return server, nil
}

// NewBaseplateServer returns a new Thrift implementation of a Baseplate
// server with the given TProcessor.
//
// The TProcessor underlying the server will be wrapped in the default
// Baseplate Middleware and any additional middleware passed in.
func NewBaseplateServer(
	bp baseplate.Baseplate,
	processor thrift.TProcessor,
	middlewares ...thrift.ProcessorMiddleware,
) (baseplate.Server, error) {
	cfg := ServerConfig{
		Addr:    bp.Config().Addr,
		Timeout: bp.Config().Timeout,
		Logger:  thrift.Logger(log.ZapWrapper(bp.Config().Log.Level)),
	}
	wrapped := BaseplateDefaultProcessorMiddlewares(bp.EdgeContextImpl())
	wrapped = append(wrapped, middlewares...)
	srv, err := NewServer(cfg, processor, wrapped...)
	if err != nil {
		return nil, err
	}
	return ApplyBaseplate(bp, srv), nil
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
