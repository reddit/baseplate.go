package thrifttest

import (
	"context"
	"time"

	"github.com/apache/thrift/lib/go/thrift"

	baseplate "github.com/reddit/baseplate.go"
	"github.com/reddit/baseplate.go/batchcloser"
	"github.com/reddit/baseplate.go/ecinterface"
	"github.com/reddit/baseplate.go/errorsbp"
	"github.com/reddit/baseplate.go/secrets"
	"github.com/reddit/baseplate.go/thriftbp"
)

const (
	// DefaultClientMaxConnections is used when ServerConfig.ClientConfig.MaxConnections
	// is not set.
	DefaultClientMaxConnections = 10

	// DefaultClientConnectTimeout is used when ServerConfig.ClientConfig.ConnectTimeout
	// is not set.
	//
	// We use a relatively large number as the default timeout because we often
	// run tests from virtual environments with very limited resources.
	DefaultClientConnectTimeout = 500 * time.Millisecond

	// DefaultClientSocketTimeout is used when ServerConfig.ClientConfig.SocketTimeout
	// is not set.
	//
	// We use a relatively large number as the default timeout because we often
	// run tests from virtual environments with very limited resources.
	DefaultClientSocketTimeout = 500 * time.Millisecond

	// DefaultServiceSlug is used when ServerConfig.ClientConfig.ServiceSlug
	// is not set.
	DefaultServiceSlug = "testing"

	// InitialClientConnections is the value that is always used for
	// ServerConfig.ClientConfig.InitialConnections.
	//
	// We default to 0 becaues the service is not started in NewBaseplateServer so
	// we do not want to try to connect to it when initializing the ClientPool.
	InitialClientConnections = 0

	// ReportClientPoolStats is the value that is always used for
	// ServerConfig.ClientConfig.ReportPoolStats.
	ReportClientPoolStats = false

	loopbackAddr = "127.0.0.1:0"
)

// ServerConfig can be used to pass in custom configuration options for the
// server and/or client created by NewBaseplateServer.
type ServerConfig struct {
	// Required, the processor to handle endpoints.
	Processor thrift.TProcessor

	// Required, the secret store.
	SecretStore *secrets.Store

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
	ClientConfig thriftbp.ClientPoolConfig

	// Optional, additional ClientMiddleware to wrap the client with.
	ClientMiddlewares []thrift.ClientMiddleware

	// Optional, additional ProcessorMiddleware to wrap the server with.
	ProcessorMiddlewares []thrift.ProcessorMiddleware

	// Optional, the ErrorSpanSuppressor used to create InjectServerSpan
	// middleware.
	ErrorSpanSuppressor errorsbp.Suppressor
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
// The server can be shut down manually using server.Close, with the shutdown
// commands defined in runtimebp, or by cancelling the given context.
func (s *Server) Start(ctx context.Context) {
	go baseplate.Serve(ctx, baseplate.ServeArgs{Server: s})
	time.Sleep(10 * time.Millisecond)
}

// Close the underying Server and Baseplate as well as the thriftbp.ClientPool.
func (s *Server) Close() error {
	closers := batchcloser.New(s.Server, s.Baseplate())
	if s.ClientPool != nil {
		closers.Add(s.ClientPool)
	}
	return closers.Close()
}

// NewBaseplateServer returns a new, Baseplate thrift server listening on the
// local loopback interface and a Baseplate ClientPool for use with that server.
//
// This is inspired by httptest.NewServer from the go standard library and can
// be used to test a thrift service.
//
// "cfg" may be empty, if it is, sane defaults will be chosen.
// The server and pool that are returned should be closed when done, but the
// Baseplate used by the server does not need to be.
//
// Here is an example usage of NewBaseplateServer:
//
//	import (
//		"context"
//		"testing"
//
//		baseplatethrift "github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
//		"github.com/reddit/baseplate.go/secrets"
//		"github.com/reddit/baseplate.go/thriftbp/thrifttest"
//	)
//
//	type BaseplateService struct {
//		Fail bool
//		Err  error
//	}
//
//	func (srv BaseplateService) IsHealthy(ctx context.Context, req *baseplatethrift.IsHealthyRequest) (r bool, err error) {
//		return !srv.Fail, srv.Err
//	}
//
//	func TestService(t *testing.T){
//		// Initialize this properly in a real test
//		var secrets *secrets.Store
//
//		ctx, cancel := context.WithCancel(context.Background())
//		defer cancel()
//
//		processor := baseplatethrift.NewBaseplateServiceV2Processor(BaseplateService{})
//		server, err := thrifttest.NewBaseplateServer(store, processor, nil, thrifttest.ServerConfig{})
//		if err != nil {
//			t.Fatal(err)
//		}
//		// cancelling the context will close the server.
//		server.Start(ctx)
//
//		client := baseplatethrift.NewBaseplateServiceV2Client(server.ClientPool)
//		success, err := client.IsHealthy(ctx)
//
//		if err != nil {
//			t.Errorf("expected no error, got %v", err)
//		}
//
//		if !success {
// 			t.Errorf("result mismatch, expected %v, got %v", c.expected.result, result)
//		}
//	}
func NewBaseplateServer(cfg ServerConfig) (*Server, error) {
	socket, err := thrift.NewTServerSocket(loopbackAddr)
	if err != nil {
		return nil, err
	}
	// Call listen to reserve a port and check for any issues early
	if err := socket.Listen(); err != nil {
		return nil, err
	}

	cfg.ServerConfig.Addr = socket.Addr().String()
	bp := baseplate.NewTestBaseplate(baseplate.NewTestBaseplateArgs{
		Config:          cfg.ServerConfig,
		Store:           cfg.SecretStore,
		EdgeContextImpl: ecinterface.Mock(),
	})
	middlewares := thriftbp.BaseplateDefaultProcessorMiddlewares(
		thriftbp.DefaultProcessorMiddlewaresArgs{
			EdgeContextImpl:     bp.EdgeContextImpl(),
			ErrorSpanSuppressor: cfg.ErrorSpanSuppressor,
		},
	)
	middlewares = append(middlewares, cfg.ProcessorMiddlewares...)
	serverCfg := thriftbp.ServerConfig{
		Socket:      socket,
		Logger:      thrift.NopLogger,
		Processor:   cfg.Processor,
		Middlewares: middlewares,
	}

	srv, err := thriftbp.NewServer(serverCfg)
	if err != nil {
		return nil, err
	}
	server := &Server{Server: thriftbp.ApplyBaseplate(bp, srv)}

	cfg.ClientConfig.Addr = server.Baseplate().GetConfig().Addr
	cfg.ClientConfig.ReportPoolStats = ReportClientPoolStats
	cfg.ClientConfig.InitialConnections = InitialClientConnections

	if cfg.ClientConfig.ConnectTimeout == 0 {
		cfg.ClientConfig.ConnectTimeout = DefaultClientConnectTimeout
	}
	if cfg.ClientConfig.SocketTimeout == 0 {
		cfg.ClientConfig.SocketTimeout = DefaultClientSocketTimeout
	}
	if cfg.ClientConfig.ServiceSlug == "" {
		cfg.ClientConfig.ServiceSlug = DefaultServiceSlug
	}
	if cfg.ClientConfig.EdgeContextImpl == nil {
		cfg.ClientConfig.EdgeContextImpl = ecinterface.Mock()
	}
	if cfg.ClientConfig.MaxConnections == 0 {
		cfg.ClientConfig.MaxConnections = DefaultClientMaxConnections
	}
	pool, err := thriftbp.NewBaseplateClientPool(cfg.ClientConfig, cfg.ClientMiddlewares...)
	if err != nil {
		server.Close()
		return nil, err
	}
	server.ClientPool = pool
	return server, nil
}
