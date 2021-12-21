package grpcbp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	pb "github.com/grpc-ecosystem/go-grpc-middleware/testing/testproto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/reddit/baseplate.go/ecinterface"
	"github.com/reddit/baseplate.go/mqsend"
	"github.com/reddit/baseplate.go/prometheusbp"
	"github.com/reddit/baseplate.go/tracing"
	"github.com/reddit/baseplate.go/transport"
)

const (
	testTimeout = time.Millisecond * 100
)

func TestInjectServerSpanInterceptorUnary(t *testing.T) {
	// create test server with InjectServerSpanInterceptor
	l, _ := setupServer(t, grpc.UnaryInterceptor(InjectServerSpanInterceptorUnary()))

	// create test client
	conn := setupClient(t, l)

	// instantiate gRPC client
	client := pb.NewTestServiceClient(conn)

	// set up recorder to validate span recording
	mmq := initTracing(t)

	t.Run("span-success", func(t *testing.T) {
		// send request to server
		if _, err := client.Ping(context.Background(), &pb.PingRequest{}); err != nil {
			t.Fatalf("Ping: %v", err)
		}

		// drain recorder to validate spans
		msg := drainRecorder(t, mmq)

		var trace tracing.ZipkinSpan
		if err := json.Unmarshal(msg, &trace); err != nil {
			t.Fatalf("recorded invalid JSON: %v", err)
		}

		got := trace.Name
		want := "Ping"
		if got != want {
			t.Errorf("got %s, want: %s", got, want)
		}

		for _, annotation := range trace.BinaryAnnotations {
			if annotation.Key == "error" {
				t.Error("got error span, want: success span")
				break
			}
		}
	})

	t.Run("span-error", func(t *testing.T) {
		// send request to server
		if _, err := client.PingError(context.Background(), &pb.PingRequest{}); err == nil {
			t.Error("PingError: got no error, want error")
		}

		// drain recorder to validate spans
		msg := drainRecorder(t, mmq)

		var trace tracing.ZipkinSpan
		if err := json.Unmarshal(msg, &trace); err != nil {
			t.Fatalf("recorded invalid JSON: %v", err)
		}

		got := trace.Name
		want := "PingError"
		if got != want {
			t.Errorf("got %s, want: %s", got, want)
		}

		for _, annotation := range trace.BinaryAnnotations {
			if annotation.Key == "error" {
				return
			}
		}
		t.Error("got success span, want: error span")
	})
}

func TestInjectEdgeContextInterceptorUnary(t *testing.T) {
	impl := ecinterface.Mock()

	// create test server with InjectServerSpanInterceptor
	l, service := setupServer(t, grpc.UnaryInterceptor(
		InjectEdgeContextInterceptorUnary(impl),
	))

	// create test client
	conn := setupClient(t, l, grpc.WithUnaryInterceptor(
		ForwardEdgeContextUnary(impl),
	))

	// instantiate gRPC client
	client := pb.NewTestServiceClient(conn)

	// create edge context
	ctx, err := impl.HeaderToContext(context.Background(), "dummy-edge-context")
	if err != nil {
		t.Fatalf("HeaderToContext: %v", err)
	}

	if _, err := client.Ping(ctx, &pb.PingRequest{}); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	ctx = service.ctx
	if ctx == nil {
		t.Error("got nil context")
	}

	if md, ok := metadata.FromIncomingContext(ctx); ok {
		value, ok := GetHeader(md, transport.HeaderEdgeRequest)
		if !ok {
			t.Fatal("header not set")
		}
		want := "dummy-edge-context"
		got := value
		if got != want {
			t.Errorf("got %s, want: %s", got, want)
		}
	}
}

func initTracing(t *testing.T) *mqsend.MockMessageQueue {
	t.Helper()

	t.Cleanup(func() {
		tracing.CloseTracer()
		tracing.InitGlobalTracer(tracing.Config{})
	})
	mmq := mqsend.OpenMockMessageQueue(mqsend.MessageQueueConfig{
		MaxQueueSize:   100,
		MaxMessageSize: 1024,
	})
	logger, startFailing := tracing.TestWrapper(t)
	tracing.InitGlobalTracer(tracing.Config{
		SampleRate:               1,
		MaxRecordTimeout:         testTimeout,
		Logger:                   logger,
		TestOnlyMockMessageQueue: mmq,
	})
	startFailing()
	return mmq
}

type mockService struct {
	ctx context.Context
	err error
}

func (t *mockService) Ping(ctx context.Context, req *pb.PingRequest) (*pb.PingResponse, error) {
	t.ctx = ctx
	return &pb.PingResponse{}, nil
}

func (t *mockService) PingError(ctx context.Context, req *pb.PingRequest) (*pb.Empty, error) {
	t.ctx = ctx
	return nil, errors.New("error")
}

func (t *mockService) PingEmpty(ctx context.Context, req *pb.Empty) (*pb.PingResponse, error) {
	panic("not implemented")
}

func (t *mockService) PingList(req *pb.PingRequest, c pb.TestService_PingListServer) error {
	panic("not implemented")
}

func (t *mockService) PingStream(c pb.TestService_PingStreamServer) error {
	if t.err != nil {
		return t.err
	}
	for {
		if _, err := c.Recv(); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if err := c.Send(&pb.PingResponse{}); err != nil {
			return err
		}
	}
}

func TestInjectPrometheusUnaryServerClientInterceptor(t *testing.T) {
	const (
		serviceName = "testSvc"
		serverName  = "testServer"
		method      = "Ping"
	)
	// create test server with InjectPrometheusUnaryServerInterceptor
	l, service := setupServer(t, grpc.UnaryInterceptor(
		InjectPrometheusUnaryServerInterceptor(serviceName),
	))

	// create test client
	conn := setupClient(t, l, grpc.WithUnaryInterceptor(
		PrometheusUnaryClientInterceptor(serviceName, serverName),
	))

	// instantiate gRPC client
	client := pb.NewTestServiceClient(conn)

	testCases := []struct {
		name    string
		wantErr codes.Code
		code    string
		success string
		method  string
	}{
		{
			name:    "success",
			wantErr: codes.OK,
			code:    "OK",
			success: "true",
			method:  "Ping",
		},
		{
			name:    "err",
			wantErr: codes.Internal,
			code:    "Unknown",
			success: "false",
			method:  "PingError",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			serverLatencyDistribution.Reset()
			serverRPCRequestCounter.Reset()
			serverActiveRequests.Reset()
			clientLatencyDistribution.Reset()
			clientRPCRequestCounter.Reset()
			clientActiveRequests.Reset()

			serverLabelValues := []string{
				serviceName,
				tt.method,
				unary,
				tt.success,
				tt.code,
			}

			serverRequestsLabelValues := []string{
				serviceName,
				tt.method,
			}

			clientLabelValues := []string{
				serviceName,
				tt.method,
				unary,
				tt.success,
				tt.code,
				serverName,
			}

			clientRequestsLabelValues := []string{
				serviceName,
				tt.method,
				serverName,
			}

			defer prometheusbp.MetricTest(t, "server latency", serverLatencyDistribution).CheckExists()
			defer prometheusbp.MetricTest(t, "server rpc count", serverRPCRequestCounter, serverLabelValues...).CheckDelta(1)
			defer prometheusbp.MetricTest(t, "server active requests", serverActiveRequests, serverRequestsLabelValues...).CheckDelta(0)
			defer prometheusbp.MetricTest(t, "client latency", clientLatencyDistribution).CheckExists()
			defer prometheusbp.MetricTest(t, "client rpc count", clientRPCRequestCounter, clientLabelValues...).CheckDelta(1)
			defer prometheusbp.MetricTest(t, "client active requests", clientActiveRequests, clientRequestsLabelValues...).CheckDelta(0)

			ctx := context.Background()
			if tt.success == "true" {
				if _, err := client.Ping(ctx, &pb.PingRequest{}); err != nil {
					t.Fatalf("Ping: %v", err)
				}
			} else {
				if _, err := client.PingError(ctx, &pb.PingRequest{}); err == nil {
					t.Fatalf("Ping: expected err got nil")
				}
			}

			ctx = service.ctx
			if ctx == nil {
				t.Error("got nil context")
			}
		})
	}
}

func TestInjectPrometheusStreamServerClientInterceptor(t *testing.T) {
	const (
		serviceName = "testSvc"
		serverName  = "testServer"
		method      = "Ping"
	)

	l, service := setupServer(t, grpc.StreamInterceptor(
		InjectPrometheusStreamServerInterceptor(serviceName),
	))

	// create test client
	conn := setupClient(t, l, grpc.WithStreamInterceptor(
		PrometheusStreamClientInterceptor(serviceName, serverName),
	))

	// instantiate gRPC client
	client := pb.NewTestServiceClient(conn)

	testCases := []struct {
		name    string
		wantErr codes.Code
		code    string
		success string
		method  string
	}{
		{
			name:    "success",
			wantErr: codes.OK,
			code:    "OK",
			success: "true",
			method:  "PingStream",
		},
		{
			name:    "err",
			wantErr: codes.NotFound,
			code:    "NotFound",
			success: "false",
			method:  "PingStream",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			serverLatencyDistribution.Reset()
			serverRPCRequestCounter.Reset()
			serverActiveRequests.Reset()
			clientLatencyDistribution.Reset()
			clientRPCRequestCounter.Reset()
			clientActiveRequests.Reset()

			if tt.success == "false" {
				service.err = status.Errorf(tt.wantErr, "test err: %v", tt.wantErr.String())
			}

			serverLabelValues := []string{
				serviceName,
				tt.method,
				serverStream,
				tt.success,
				tt.code,
			}

			serverRequestsLabelValues := []string{
				serviceName,
				tt.method,
			}

			clientLabelValues := []string{
				serviceName,
				"/mwitkow.testproto.TestService/" + tt.method,
				clientStream,
				tt.success,
				tt.code,
				serverName,
			}

			clientRequestsLabelValues := []string{
				serviceName,
				tt.method,
				serverName,
			}

			defer prometheusbp.MetricTest(t, "server latency", serverLatencyDistribution).CheckExists()
			defer prometheusbp.MetricTest(t, "server rpc count", serverRPCRequestCounter, serverLabelValues...).CheckDelta(1)
			defer prometheusbp.MetricTest(t, "server active requests", serverActiveRequests, serverRequestsLabelValues...).CheckDelta(0)
			defer prometheusbp.MetricTest(t, "client latency", clientLatencyDistribution).CheckExists()
			defer prometheusbp.MetricTest(t, "client rpc count", clientRPCRequestCounter, clientLabelValues...).CheckDelta(1)
			defer prometheusbp.MetricTest(t, "client active requests", clientActiveRequests, clientRequestsLabelValues...).CheckDelta(0)

			ctx := context.Background()
			clientStream, err := client.PingStream(ctx)
			if err != nil {
				t.Fatalf("PingStream: %v", err)
			}

			var count int
			for {
				switch {
				case count < 2:
					err = clientStream.Send(&pb.PingRequest{})
				default:
					err = clientStream.CloseSend()
				}
				if err != nil && err != io.EOF {
					t.Fatalf("clientStream Send or CloseSend: %v", err)
				}
				if err == io.EOF {
					break
				}

				_, err := clientStream.Recv()
				if err == io.EOF {
					break
				}

				if got, want := tt.success, fmt.Sprint(err == nil || err == io.EOF); got != want {
					t.Errorf("tt.success = %q, want %q (err: %v)", got, want, err)
				}

				count++
			}
		})
	}
}
