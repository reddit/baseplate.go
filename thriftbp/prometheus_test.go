package thriftbp

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
)

func TestPrometheusMetricsMiddleware(t *testing.T) {
	testCases := []struct {
		name          string
		wantErr       thrift.TException
		wantOK        bool
		baseplateCode string
	}{
		{"bp error", thrift.NewTProtocolExceptionWithType(thrift.PROTOCOL_ERROR, WrapBaseplateError(errors.New("test"))), false, ""},
		{"application error", thrift.NewTApplicationException(thrift.UNKNOWN_METHOD, "unknown err msg"), false, ""},
		{"compile error", baseplate.NewError(), false, "0"},
		{"success", nil, true, ""},
	}

	const (
		serviceName = "testservice"
		method      = "testmethod"
	)
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			latencyDistribution.Reset()
			rpcStatusCounter.Reset()
			activeRequests.Reset()

			next := thrift.WrappedTProcessorFunction{
				Wrapped: func(ctx context.Context, seqId int32, in, out thrift.TProtocol) (bool, thrift.TException) {
					return tt.wantOK, tt.wantErr
				},
			}
			promMiddleware := PrometheusMetricMiddleware(serviceName)
			wrapped := promMiddleware(method, next)
			gotOK, gotErr := wrapped.Process(context.Background(), 1, nil, nil)

			if gotOK != tt.wantOK {
				t.Errorf("wanted %v, got %v", tt.wantOK, gotOK)
			}
			if gotErr != tt.wantErr {
				t.Errorf("wanted %v, got %v", tt.wantErr, gotErr)
			}

			var baseplateCodeStatus string
			var exceptionType string
			if gotErr != nil {
				exceptionType = strings.TrimPrefix(fmt.Sprintf("%T", gotErr), "*")
			}
			success := strconv.FormatBool(gotErr == nil)
			thriftLabelValues := []string{
				serviceName,
				method,
				success,
				exceptionType,
				baseplateCodeStatus,
				tt.baseplateCode,
			}
			latencyMetricCount := testutil.CollectAndCount(latencyDistribution)
			if latencyMetricCount != 1 {
				t.Errorf("wanted %v, got %v", 1, latencyMetricCount)
			}

			rpcMetricValue := testutil.ToFloat64(rpcStatusCounter.WithLabelValues(thriftLabelValues...))
			if rpcMetricValue != 1 {
				t.Errorf("wanted %v, got %v", 0, rpcMetricValue)
			}
			rpcMetricCount := testutil.CollectAndCount(rpcStatusCounter)
			if rpcMetricCount != 1 {
				t.Errorf("wanted %v, got %v", 1, rpcMetricCount)
			}

			requestLabelValues := []string{
				serviceName,
				method,
			}
			requestMetricValue := testutil.ToFloat64(activeRequests.WithLabelValues(requestLabelValues...))
			if requestMetricValue != 0 {
				t.Errorf("wanted %v, got %v", 0, requestMetricValue)
			}
			requestMetricCount := testutil.CollectAndCount(activeRequests)
			if requestMetricCount != 1 {
				t.Errorf("wanted %v, got %v", 1, requestMetricCount)
			}
		})
	}
}
