package grpcbp

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	pb "github.com/grpc-ecosystem/go-grpc-middleware/testing/testproto"
	"github.com/opentracing/opentracing-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"

	"github.com/reddit/baseplate.go/mqsend"
	"github.com/reddit/baseplate.go/tracing"
)

func TestMonitorInterceptorUnary(t *testing.T) {
	// create test server
	listener, _ := setupServer(t)

	// create test client with monitoring interceptor
	conn := setupClient(t, listener, grpc.WithUnaryInterceptor(
		MonitorInterceptorUnary(
			MonitorInterceptorArgs{
				ServiceSlug: "test",
			},
		),
	))

	// instantiate gRPC client
	ctx, mmq := setupServerSpan(t)
	client := pb.NewTestServiceClient(conn)

	t.Run("span-success", func(t *testing.T) {
		if _, err := client.Ping(ctx, &pb.PingRequest{}); err != nil {
			t.Fatalf("Ping: %v", err)
		}

		// drain recorder to validate spans
		msg := drainRecorder(t, mmq)

		var trace tracing.ZipkinSpan
		if err := json.Unmarshal(msg, &trace); err != nil {
			t.Fatalf("recorded invalid JSON: %v", err)
		}

		got := trace.Name
		want := "test.Ping"
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
		_, err := client.PingError(ctx, &pb.PingRequest{})
		if err == nil {
			t.Error("got no error, want error")
		}

		// drain recorder to validate spans
		msg := drainRecorder(t, mmq)

		var trace tracing.ZipkinSpan
		if err := json.Unmarshal(msg, &trace); err != nil {
			t.Fatalf("recorded invalid JSON: %v", err)
		}

		got := trace.Name
		want := "test.PingError"
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

func setupServer(t *testing.T, opts ...grpc.ServerOption) (*bufconn.Listener, *mockService) {
	t.Helper()

	// create in-memory connection to reproduce network behavior but without
	// the need to clean up OS-level resources
	l := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer(opts...)
	service := &mockService{}
	pb.RegisterTestServiceServer(s, service)
	go func() {
		if err := s.Serve(l); err != nil {
			t.Error(err)
		}
	}()
	t.Cleanup(s.Stop)

	return l, service
}

func setupClient(t *testing.T, l *bufconn.Listener, opts ...grpc.DialOption) *grpc.ClientConn {
	t.Helper()

	bufDialer := func(ctx context.Context, _ string) (net.Conn, error) {
		return l.DialContext(ctx)
	}
	opts = append([]grpc.DialOption{
		grpc.WithContextDialer(bufDialer),
		grpc.WithInsecure(),
	}, opts...)

	// create connection to be used by gRPC client
	conn, err := grpc.DialContext(
		context.Background(),
		"bufnet",
		opts...,
	)
	if err != nil {
		t.Fatalf("DialContext: %v", err)
	}
	t.Cleanup(func() {
		err := conn.Close()
		if err != nil {
			t.Fatalf("close connection: %v", err)
		}
	})

	return conn
}

func setupServerSpan(t *testing.T) (context.Context, *mqsend.MockMessageQueue) {
	t.Helper()

	recorder := mqsend.OpenMockMessageQueue(mqsend.MessageQueueConfig{
		MaxQueueSize:   100,
		MaxMessageSize: 1024,
	})
	tracing.InitGlobalTracer(tracing.Config{
		SampleRate:               1.0,
		TestOnlyMockMessageQueue: recorder,
	})

	span, ctx := opentracing.StartSpanFromContext(
		context.Background(),
		"test-service",
		tracing.SpanTypeOption{Type: tracing.SpanTypeServer},
	)
	tracing.AsSpan(span).SetDebug(true)
	return ctx, recorder
}

func drainRecorder(t *testing.T, recorder *mqsend.MockMessageQueue) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	msg, err := recorder.Receive(ctx)
	if err != nil {
		t.Fatalf("draining recorder: %v", err)
	}
	return msg
}
