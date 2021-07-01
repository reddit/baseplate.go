package thriftbp

import (
	"time"

	"github.com/apache/thrift/lib/go/thrift"

	baseplate "github.com/reddit/baseplate.go"
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

	// Optional, used only by NewServer.
	// In NewBaseplateServer the timeout set in bp.Config() will be used instead.
	//
	// The timeout for the underlying thrift.TServerSocket transport.
	//
	// If your clients use client pool,
	// it's recommended to either not set a Timeout on your server,
	// or set it to a long value that's longer than your clients' pool TTL.
	//
	// This is ignored if Socket is non-nil.
	Timeout time.Duration

	// Optional, used only by NewServer.
	// In NewBaseplateServer the address and timeout set in bp.Config() will be
	// used instead.
	//
	// You can choose to set Socket instead of Addr plus Timeout.
	Socket *thrift.TServerSocket
}

// NewServer returns a thrift.TSimpleServer using the THeader transport
// and protocol to serve the given TProcessor which is wrapped with the
// given ProcessorMiddlewares.
func NewServer(cfg ServerConfig) (*thrift.TSimpleServer, error) {
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
	middlewares := BaseplateDefaultProcessorMiddlewares(
		DefaultProcessorMiddlewaresArgs{
			EdgeContextImpl:                    bp.EdgeContextImpl(),
			ErrorSpanSuppressor:                cfg.ErrorSpanSuppressor,
			ReportPayloadSizeMetricsSampleRate: cfg.ReportPayloadSizeMetricsSampleRate,
		},
	)
	middlewares = append(middlewares, cfg.Middlewares...)
	cfg.Middlewares = middlewares
	cfg.Logger = log.ZapWrapper(bp.GetConfig().Log.Level).ToThriftLogger()
	cfg.Addr = bp.GetConfig().Addr
	cfg.Timeout = bp.GetConfig().Timeout
	cfg.Socket = nil
	srv, err := NewServer(cfg)
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
