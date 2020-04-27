package thriftbp

import (
	"time"

	"github.com/apache/thrift/lib/go/thrift"
)

// ServerConfig is the arg struct for NewServer.
type ServerConfig struct {
	// The endpoint address of your thrift service
	Addr string

	// The timeout for the underlying thrift.TServerSocket transport.
	Timeout time.Duration

	// A log wrapper that is used by the TSimpleServer.
	//
	// It's compatible with log.Wrapper (with an extra typecasting),
	// but you should not use log.ErrorWithSentryWrapper for this one,
	// as it would log all the network I/O errors,
	// which would be too spammy for sentry.
	Logger thrift.Logger
}

// NewServer returns a thrift.TSimpleServer using the THeader transport
// and protocol to serve the given TProcessor which is wrapped with the
// given ProcessorMiddlewares.
func NewServer(
	cfg ServerConfig,
	processor thrift.TProcessor,
	middlewares ...thrift.ProcessorMiddleware,
) (*thrift.TSimpleServer, error) {
	transport, err := thrift.NewTServerSocketTimeout(cfg.Addr, cfg.Timeout)
	if err != nil {
		return nil, err
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
