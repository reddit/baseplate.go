package thriftbp

import (
	"strings"
	"time"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/reddit/baseplate.go"
	"github.com/reddit/baseplate.go/errorsbp"

	//lint:ignore SA1019 This library is internal only, not actually deprecated
	"github.com/reddit/baseplate.go/internalv2compat"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/metricsbp"
)

const (
	meterNameThriftSocketErrorCounter = "thrift.socket.timeout"
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

	// Optional, used by NewBaseplateServer and NewServer.
	//
	// Report the number of clients connected to the server as a runtime gauge
	// with metric name of 'thrift.connections'
	ReportConnectionCount bool

	// Optional, used only by NewServer.
	// In NewBaseplateServer the address set in bp.Config() will be used instead.
	//
	// The endpoint address of your thrift service.
	//
	// This is ignored if Socket is non-nil.
	Addr string

	// Deprecated: No-op for now, will be removed in a future release.
	Timeout time.Duration

	// Optional, This duration is used to set both the read and write idle timeouts
	// for the thrift.TServerSocket used by the baseplate server.
	//
	// This is an experimental configuration and is subject to change or deprecation
	// without notice. When using NewBaseplateServer, setting a socket timeout will
	// also override the default thrift server logger to one that emits metrics
	// instead of logs in the event of a socket disconnect. A zero value means I/O
	// read or write operations will not time out.
	SocketTimeout time.Duration

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
		if cfg.SocketTimeout > 0 {
			transport, err = thrift.NewTServerSocketTimeout(cfg.Addr, cfg.SocketTimeout)
		} else {
			transport, err = thrift.NewTServerSocket(cfg.Addr)
		}
		if err != nil {
			return nil, err
		}
	} else {
		transport = cfg.Socket
	}

	if cfg.ReportConnectionCount {
		transport = &CountedTServerTransport{transport}
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

	cfg.Logger = log.ZapWrapper(log.ZapWrapperArgs{
		Level: bp.GetConfig().Log.Level,
		KVPairs: map[string]interface{}{
			"from": "thrift",
		},
	}).ToThriftLogger()

	if cfg.SocketTimeout > 0 {
		cfg.Logger = suppressTimeoutLogger(cfg.Logger)
	}

	cfg.Addr = bp.GetConfig().Addr
	cfg.Socket = nil
	srv, err := NewServer(cfg)
	if err != nil {
		return nil, err
	}
	return ApplyBaseplate(bp, srv), nil
}

func suppressTimeoutLogger(logger thrift.Logger) thrift.Logger {
	c := metricsbp.M.Counter(meterNameThriftSocketErrorCounter)
	return func(msg string) {
		if strings.Contains(msg, "i/o timeout") {
			c.Add(1)
			return
		}

		logger(msg)
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

	internalv2compat.IsThrift
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
