package thriftbp

import (
	"time"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/reddit/baseplate.go/log"
)

// ServerConfig is the arg struct for NewThriftServer.
type ServerConfig struct {
	// The endpoint address of your thrift service
	Addr string

	// The timeout for the underlying thrift.TServerSocket transport.
	Timeout time.Duration

	// A log wrapper that is used by the TSimpleServer.
	Logger log.Wrapper
}

// NewThriftServer returns a thrift.TSimpleServer using the THeader transport
// and protocol to serve the given BaseplateProcessor which is wrapped with the
// given Middlewares.
func NewThriftServer(cfg ServerConfig, processor BaseplateProcessor, middlewares ...Middleware) (*thrift.TSimpleServer, error) {
	transport, err := thrift.NewTServerSocketTimeout(cfg.Addr, cfg.Timeout)
	if err != nil {
		return nil, err
	}

	server := thrift.NewTSimpleServer4(
		Wrap(processor, middlewares...),
		transport,
		thrift.NewTHeaderTransportFactory(nil),
		thrift.NewTHeaderProtocolFactory(),
	)
	server.SetForwardHeaders(HeadersToForward)
	server.SetLogger(thrift.Logger(cfg.Logger))
	return server, nil
}
