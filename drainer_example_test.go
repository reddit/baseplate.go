package baseplate_test

import (
	"context"
	"flag"
	"io"

	baseplate "github.com/reddit/baseplate.go"
	baseplatethrift "github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/thriftbp"
)

// A placeholder thrift service for the example.
type Service struct {
	drainer baseplate.HealthCheckCloser
}

func (s *Service) IsHealthy(ctx context.Context, req *baseplatethrift.IsHealthyRequest) (bool, error) {
	switch req.GetProbe() {
	default:
		// For unknown probes, default to readiness.
		fallthrough
	case baseplatethrift.IsHealthyProbe_READINESS:
		return s.drainer.IsHealthy(ctx) /* && other healthy dependencies */, nil

	case baseplatethrift.IsHealthyProbe_LIVENESS:
		return true /* && other healthy dependencies */, nil
	}
}

// This example demonstrates how to use baseplate.Drainer in your main function
// and service's IsHealthy handler.
func ExampleDrainer() {
	flag.Parse()
	ctx, bp, err := baseplate.New(context.Background(), baseplate.NewArgs{
		// TODO: fill in NewArgs.
	})
	if err != nil {
		log.Fatal(err)
	}
	defer bp.Close()

	// TODO: Other initializations
	drainer := baseplate.Drainer()
	// TODO: Other initializations

	processor := baseplatethrift.NewBaseplateServiceV2Processor(&Service{
		drainer: drainer,
	})
	server, err := thriftbp.NewBaseplateServer(bp, thriftbp.ServerConfig{
		Processor: processor,
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Info(baseplate.Serve(ctx, baseplate.ServeArgs{
		Server: server,
		PreShutdown: []io.Closer{
			drainer,
			// TODO: Other pre-shutdown closers
		},
		PostShutdown: []io.Closer{
			// TODO: Post-shutdown closers
		},
	}))
}
