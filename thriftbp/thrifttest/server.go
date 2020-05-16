package thrifttest

import (
	"context"
	"time"

	"github.com/apache/thrift/lib/go/thrift"

	baseplate "github.com/reddit/baseplate.go"
	"github.com/reddit/baseplate.go/batcherror"
	"github.com/reddit/baseplate.go/secrets"
	"github.com/reddit/baseplate.go/thriftbp"
)

const loopbackAddr = "127.0.0.1:0"

// ServerConfig can be used to pass in custom configuration options for the
// server and/or client created by NewBaseplateServer.
type ServerConfig struct {
	// ServerConfig is an optional value, sane defaults will be chosen where
	// appropriate.
	//
	// ServerConfig.Socket will always be replaced with one created in
	// NewBaseplateServer using the local loopback address.
	ServerConfig baseplate.Config

	// ClientConfig is an optional value, sane defaults will be chosen where
	// appropriate.
	//
	// Addr will always be set to the address of the test server.
	// ReportPoolStats will always be set to false.
	// InitialConnections will always be set to 0 since the test server will not
	// be started yet.
	// MaxConnections will be 10 if it is not set.
	ClientConfig thriftbp.ClientPoolConfig

	// Optional, additional ClientMiddleware to wrap the client with.
	ClientMiddlewares []thrift.ClientMiddleware

	// Optional, additional ProcessorMiddleware to wrap the server with.
	ProcessorMiddlewares []thrift.ProcessorMiddleware
}

// Server is a test server returned by NewBaseplateServer.  It contains both
// the baseplate.Server and a ClientPool to use to interact with the server.
//
// Server implements baseplate.Server.
type Server struct {
	baseplate.Server

	// ClientPool provides a thriftbp.ClientPool that connects to this Server and
	// can be used for making Thrift client objects to interact with this Server.
	ClientPool thriftbp.ClientPool
}

// Start starts the server using baseplate.Serve in a background goroutine and
// waits for a short period of time to give the server time to start up.
//
// The server can be closed manually, with shutdown commands, or by cancelling
// the given context.
func (s *Server) Start(ctx context.Context) {
	go baseplate.Serve(ctx, s)
	time.Sleep(10 * time.Millisecond)
}

// Close both the underying baseplate.Server and thriftbp.ClientPool
func (s *Server) Close() error {
	var errs batcherror.BatchError
	if err := s.Server.Close(); err != nil {
		errs.Add(err)
	}
	if s.ClientPool != nil {
		if err := s.ClientPool.Close(); err != nil {
			errs.Add(err)
		}
	}
	return errs.Compile()
}

// NewBaseplateServer returns a new, Baseplate thrift server listening on the
// local loopback interface and a Baseplate ClientPool for use with that server.
//
// This is inspired by httptest.NewServer from the go standard library and can
// be used to test a thrift service.
//
// "cfg" may be nil, if it is, sane defaults will be chosen.
// The server and pool that are returned should be closed when done, but the
// Baseplate used by the server does not need to be.
func NewBaseplateServer(
	store *secrets.Store,
	processor thrift.TProcessor,
	cfg *ServerConfig,
) (*Server, error) {
	if cfg == nil {
		cfg = &ServerConfig{}
	}

	socket, err := thrift.NewTServerSocket(loopbackAddr)
	if err != nil {
		return nil, err
	}
	// Call listen to reserve a port and check for any issues early
	if err := socket.Listen(); err != nil {
		return nil, err
	}

	cfg.ServerConfig.Addr = socket.Addr().String()
	bp := baseplate.NewTestBaseplate(cfg.ServerConfig, store)
	middlewares := thriftbp.BaseplateDefaultProcessorMiddlewares(bp.EdgeContextImpl())
	middlewares = append(middlewares, cfg.ProcessorMiddlewares...)
	serverCfg := thriftbp.ServerConfig{
		Socket: socket,
		Logger: thrift.NopLogger,
	}

	srv, err := thriftbp.NewServer(serverCfg, processor, middlewares...)
	if err != nil {
		return nil, err
	}
	server := &Server{Server: thriftbp.ApplyBaseplate(bp, srv)}

	cfg.ClientConfig.Addr = server.Baseplate().Config().Addr
	cfg.ClientConfig.ReportPoolStats = false
	cfg.ClientConfig.InitialConnections = 0
	if cfg.ClientConfig.SocketTimeout == 0 {
		cfg.ClientConfig.SocketTimeout = 10 * time.Millisecond
	}
	if cfg.ClientConfig.ServiceSlug == "" {
		cfg.ClientConfig.ServiceSlug = "testing"
	}
	if cfg.ClientConfig.MaxConnections == 0 {
		cfg.ClientConfig.MaxConnections = 10
	}
	pool, err := thriftbp.NewBaseplateClientPool(cfg.ClientConfig, cfg.ClientMiddlewares...)
	if err != nil {
		server.Close()
		return nil, err
	}
	server.ClientPool = pool
	return server, nil
}
