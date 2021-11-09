package thriftbp

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
	"github.com/reddit/baseplate.go/prometheusbp"
)

func TestPrometheusServerMiddleware(t *testing.T) {
	testCases := []struct {
		name          string
		wantErr       thrift.TException
		wantOK        bool
		baseplateCode string
	}{
		{
			name:          "bp error",
			wantErr:       thrift.NewTProtocolExceptionWithType(thrift.PROTOCOL_ERROR, WrapBaseplateError(errors.New("test"))),
			wantOK:        false,
			baseplateCode: "",
		},
		{
			name:          "application error",
			wantErr:       thrift.NewTApplicationException(thrift.UNKNOWN_METHOD, "unknown err msg"),
			wantOK:        false,
			baseplateCode: "",
		},
		{
			name:          "compile error",
			wantErr:       baseplate.NewError(),
			wantOK:        false,
			baseplateCode: "0",
		},
		{
			name:          "success",
			wantErr:       nil,
			wantOK:        true,
			baseplateCode: "",
		},
	}

	const (
		serviceName = "testservice"
		method      = "testmethod"
	)
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			serverLatencyDistribution.Reset()
			serverRPCRequestCounter.Reset()
			serverActiveRequests.Reset()

			var baseplateCodeStatus string
			var exceptionType string
			if tt.wantErr != nil {
				exceptionType = strings.TrimPrefix(fmt.Sprintf("%T", tt.wantErr), "*")
			}

			success := strconv.FormatBool(tt.wantErr == nil)
			labelValues := []string{
				serviceName,
				method,
				success,
				exceptionType,
				baseplateCodeStatus,
				tt.baseplateCode,
			}

			requestLabelValues := []string{
				serviceName,
				method,
			}

			defer prometheusbp.MetricTest(t, "latency", serverLatencyDistribution).CheckExists()
			defer prometheusbp.MetricTest(t, "rpc count", serverRPCRequestCounter, labelValues...).CheckDelta(1)
			defer prometheusbp.MetricTest(t, "active requests", serverActiveRequests, requestLabelValues...).CheckDelta(0)

			next := thrift.WrappedTProcessorFunction{
				Wrapped: func(ctx context.Context, seqId int32, in, out thrift.TProtocol) (bool, thrift.TException) {
					return tt.wantOK, tt.wantErr
				},
			}
			promMiddleware := PrometheusServerMiddleware(serviceName)
			wrapped := promMiddleware(method, next)
			gotOK, gotErr := wrapped.Process(context.Background(), 1, nil, nil)

			if gotOK != tt.wantOK {
				t.Errorf("wanted %v, got %v", tt.wantOK, gotOK)
			}
			if gotErr != tt.wantErr {
				t.Errorf("wanted %v, got %v", tt.wantErr, gotErr)
			}
		})
	}
}

func TestPrometheusClientMiddleware(t *testing.T) {
	clientLatencyDistribution.Reset()
	clientRPCRequestCounter.Reset()
	clientActiveRequests.Reset()

	const (
		service = "testService"
		server  = "testServer"
		method  = "testMethod"
	)

	labelValues := []string{
		service,
		method,
		"true",
		"",
		"",
		"",
		server,
	}

	requestLabelValues := []string{
		service,
		method,
	}

	defer prometheusbp.MetricTest(t, "latency", clientLatencyDistribution).CheckExists()
	defer prometheusbp.MetricTest(t, "rpc count", clientRPCRequestCounter, labelValues...).CheckDelta(1)
	defer prometheusbp.MetricTest(t, "active requests", clientActiveRequests, requestLabelValues...).CheckDelta(0)

	client := thrift.WrapClient(mockClient{}, PrometheusClientMiddleware(service, server))
	if _, err := client.Call(context.Background(), method, nil, nil); err != nil {
		t.Fatal(err)
	}
}

type mockClient struct {
}

func (c mockClient) Call(ctx context.Context, method string, args, result thrift.TStruct) (thrift.ResponseMeta, error) {
	return thrift.ResponseMeta{}, nil
}

var _ thrift.TClient = (*mockClient)(nil)
