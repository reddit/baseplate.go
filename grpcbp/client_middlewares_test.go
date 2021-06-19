package grpcbp_test

import (
	"context"
	"testing"

	grpc_testing "github.com/grpc-ecosystem/go-grpc-middleware/testing"
	pb_testproto "github.com/grpc-ecosystem/go-grpc-middleware/testing/testproto"
	"github.com/reddit/baseplate.go/tracing"
)

type traceService struct {
	pb_testproto.TestServiceServer
	pingMethod func(context.Context, *pb_testproto.PingRequest) (*pb_testproto.PingResponse, error)
}

func TestTraceInterceptor(t *testing.T) {
	cases := []struct {
		name       string
		pingMethod func(context.Context, *pb_testproto.PingRequest) (*pb_testproto.PingResponse, error)
	}{
		{
			name: "server span: success",
			pingMethod: func(context.Context, *pb_testproto.PingRequest) (*pb_testproto.PingResponse, error) {
				return &pb_testproto.PingResponse{}, nil
			},
		},
	}

	for _, c := range cases {
		t.Run(
			c.name,
			func(t *testing.T) {
				defer func() {
					tracing.CloseTracer()
					tracing.InitGlobalTracer(tracing.TracerConfig{})
				}()
				service := &traceService{
					TestServiceServer: &grpc_testing.TestPingService{T: t},
					pingMethod:        c.pingMethod,
				}
				_, err := service.Ping(context.Background(), &pb_testproto.PingRequest{})
				if err != nil {
					t.Fatal(err)
				}
			},
		)
	}
}
