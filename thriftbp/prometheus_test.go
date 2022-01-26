package thriftbp

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
	"github.com/reddit/baseplate.go/internal/prometheusbp/spectest"
	"github.com/reddit/baseplate.go/prometheusbp/promtest"
)

func TestPrometheusServerMiddleware(t *testing.T) {
	testCases := []struct {
		name                string
		wantErr             thrift.TException
		wantOK              bool
		baseplateStatusCode string
	}{
		{
			name:                "bp error",
			wantErr:             thrift.NewTProtocolExceptionWithType(thrift.PROTOCOL_ERROR, WrapBaseplateError(errors.New("test"))),
			wantOK:              false,
			baseplateStatusCode: "",
		},
		{
			name:                "application error",
			wantErr:             thrift.NewTApplicationException(thrift.UNKNOWN_METHOD, "unknown err msg"),
			wantOK:              false,
			baseplateStatusCode: "",
		},
		{
			name:                "compile error",
			wantErr:             baseplate.NewError(),
			wantOK:              false,
			baseplateStatusCode: "0",
		},
		{
			name:                "success",
			wantErr:             nil,
			wantOK:              true,
			baseplateStatusCode: "",
		},
	}

	const method = "testmethod"

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			serverLatencyDistribution.Reset()
			serverTotalRequests.Reset()
			serverActiveRequests.Reset()

			var baseplateStatus string
			var exceptionType string
			if tt.wantErr != nil {
				exceptionType = strings.TrimPrefix(fmt.Sprintf("%T", tt.wantErr), "*")
			}

			success := strconv.FormatBool(tt.wantErr == nil)

			activeRequestLabels := prometheus.Labels{
				methodLabel: method,
			}

			latencyLabels := prometheus.Labels{
				methodLabel:  method,
				successLabel: success,
			}

			totalRequestLabels := prometheus.Labels{
				methodLabel:              method,
				successLabel:             success,
				exceptionLabel:           exceptionType,
				baseplateStatusLabel:     baseplateStatus,
				baseplateStatusCodeLabel: tt.baseplateStatusCode,
			}

			defer promtest.NewPrometheusMetricTest(t, "latency", serverLatencyDistribution, latencyLabels).CheckExists()
			defer promtest.NewPrometheusMetricTest(t, "rpc count", serverTotalRequests, totalRequestLabels).CheckDelta(1)
			defer promtest.NewPrometheusMetricTest(t, "active requests", serverActiveRequests, activeRequestLabels).CheckDelta(0)
			defer spectest.ValidateSpec(t, "thrift", "server")

			next := thrift.WrappedTProcessorFunction{
				Wrapped: func(ctx context.Context, seqId int32, in, out thrift.TProtocol) (bool, thrift.TException) {
					return tt.wantOK, tt.wantErr
				},
			}
			gotOK, gotErr := PrometheusServerMiddleware(method, next).Process(context.Background(), 1, nil, nil)

			if gotOK != tt.wantOK {
				t.Errorf("wanted %v, got %v", tt.wantOK, gotOK)
			}
			if gotErr != tt.wantErr {
				t.Errorf("wanted %v, got %v", tt.wantErr, gotErr)
			}
		})
	}
}

// PromClientMetricsTest keeps track of the Thrift client Prometheus metrics
// during testing.
type PromClientMetricsTest struct {
	tb testing.TB

	latency        *promtest.PrometheusMetricTest
	totalRequests  *promtest.PrometheusMetricTest
	activeRequests *promtest.PrometheusMetricTest
}

// PrometheusClientMetricsTest resets the Thrift client Prometheus metrics and
// setups the test to track the client metrics.
func PrometheusClientMetricsTest(tb testing.TB, latencyLabelValues, requestCountLabelValues, activeRequestsLabelValues prometheus.Labels) PromClientMetricsTest {
	tb.Helper()

	clientLatencyDistribution.Reset()
	clientTotalRequests.Reset()
	clientActiveRequests.Reset()
	return PromClientMetricsTest{
		tb:             tb,
		latency:        promtest.NewPrometheusMetricTest(tb, "latency", clientLatencyDistribution, latencyLabelValues),
		totalRequests:  promtest.NewPrometheusMetricTest(tb, "rpc count", clientTotalRequests, requestCountLabelValues),
		activeRequests: promtest.NewPrometheusMetricTest(tb, "active requests", clientActiveRequests, activeRequestsLabelValues),
	}
}

// CheckMetrics ensure the correct client metrics were registered and tracked
// for Thrift client Prometheus metrics.
func (p PromClientMetricsTest) CheckMetrics(requests int) {
	p.tb.Helper()

	p.latency.CheckExistsN(requests)
	p.totalRequests.CheckDelta(1)
	p.activeRequests.CheckDelta(0)
}
