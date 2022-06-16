package grpcbp

import (
	"net"
	"time"

	"github.com/reddit/baseplate.go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

// ServerConfig is the argument struct for NewBaseplateServer. Please refer to
// the documentation for each field to see how is it used.
type ServerConfig struct {
	// MaxConnectionIdle is a duration for the amount of time after which an idle
	// connection would be closed by sending a GoAway. Idleness duration is
	// defined since the most recent time the number of outstanding RPCs became
	// zero or the connection establishment. The current default value is
	// infinity.
	MaxConnectionIdle time.Duration `yaml:"maxConnectionIdle"`

	// MaxConnectionAge is a duration for the maximum amount of time a
	// connection may exist before it will be closed by sending a GoAway. A
	// random jitter of +/-10% will be added to MaxConnectionAge to spread out
	// connection storms. The current default value is infinity.
	MaxConnectionAge time.Duration `yaml:"maxConnectionAge"`

	// MaxConnectionAgeGrace is an additive period after MaxConnectionAge after
	// which the connection will be forcibly closed. The current defualt value is
	// infinity.
	MaxConnectionAgeGrace time.Duration `yaml:"maxConnectionAgeGrace"`

	// After a duration of this time if the server doesn't see any activity it
	// pings the client to see if the transport is still alive. If set below 1s,
	// a minimum value of 1s will be used instead. The current default value is 2
	// hours.
	Time time.Duration `yaml:"time"`

	// After having pinged for keepalive check, the server waits for a duration
	// of Timeout and if no activity is seen even after that the connection is
	// closed. The current default value is 20 seconds.
	Timeout time.Duration `yaml:"timeout"`

	// MinTime is the minimum amount of time a client should wait before sending
	// a keepalive ping.
	MinTime time.Duration `yaml:"minTime"`

	// If true, server allows keepalive pings even when there are no active
	// streams(RPCs). If false, and client sends ping when there are no active
	// streams, server will send GOAWAY and close the connection.
	PermitWithoutStream bool `yaml:"permitWithoutStream"`

	// RegisterServerFunc is used to pass in a generated gRPC service
	// implementation and register it on the created gRPC.Server.
	RegisterServerFunc func(*grpc.Server) `yaml:"-"`

	// AdditionalServerOptions is used to pass in additional server options
	// to use when constructing a
	AdditionalServerOptions []grpc.ServerOption `yaml:"-"`
}

// NewBaseplateServer returns a new gRPC implementation of a Baseplate server
// with the given config.
func NewBaseplateServer(bp baseplate.Baseplate, cfg ServerConfig) (baseplate.Server, error) {
	lis, err := net.Listen("tcp", bp.GetConfig().Addr)
	if err != nil {
		return nil, err
	}

	kaep := keepalive.EnforcementPolicy{
		MinTime:             cfg.MinTime,
		PermitWithoutStream: cfg.PermitWithoutStream,
	}

	kasp := keepalive.ServerParameters{
		MaxConnectionIdle:     cfg.MaxConnectionIdle,
		MaxConnectionAge:      cfg.MaxConnectionAge,
		MaxConnectionAgeGrace: cfg.MaxConnectionAgeGrace,
		Time:                  cfg.Time,
		Timeout:               cfg.Timeout,
	}

	middlewares := BaseplateDefaultMiddlewares(DefaultMiddlewaresArgs{
		EdgeContextImpl: bp.EdgeContextImpl(),
	})

	options := []grpc.ServerOption{
		grpc.KeepaliveEnforcementPolicy(kaep),
		grpc.KeepaliveParams(kasp),
		middlewares,
	}

	if len(cfg.AdditionalServerOptions) > 0 {
		options = append(options, cfg.AdditionalServerOptions...)
	}

	server := grpc.NewServer(options...)
	cfg.RegisterServerFunc(server)

	return ApplyBaseplate(bp, server, lis), nil
}

// ApplyBaseplate returns the given grpc.Server as a baseplate server with the
// provided Baseplate.
//
// You generally don't need to use this, instead use NewBaseplateServer, which
// will take care of this for you.
func ApplyBaseplate(bp baseplate.Baseplate, server *grpc.Server, lis net.Listener) baseplate.Server {
	return impl{
		bp:  bp,
		lis: lis,
		srv: server,
	}
}

type impl struct {
	bp  baseplate.Baseplate
	srv *grpc.Server
	lis net.Listener
}

func (s impl) Baseplate() baseplate.Baseplate {
	return s.bp
}

func (s impl) Serve() error {
	return s.srv.Serve(s.lis)
}

func (s impl) Close() error {
	s.srv.GracefulStop()
	return nil
}

var (
	_ baseplate.Server = impl{}
	_ baseplate.Server = (*impl)(nil)
)
