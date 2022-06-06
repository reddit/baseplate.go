package thriftbp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/reddit/baseplate.go/clientpool"
	"github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
	"github.com/reddit/baseplate.go/internal/prometheusbp/spectest"
	"github.com/reddit/baseplate.go/prometheusbp"
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

			success := prometheusbp.BoolString(tt.wantErr == nil)

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

			defer promtest.NewPrometheusMetricTest(t, "latency", serverLatencyDistribution, latencyLabels).CheckSampleCountDelta(1)
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
func (p PromClientMetricsTest) CheckMetrics() {
	p.tb.Helper()

	p.latency.CheckSampleCountDelta(1)
	p.totalRequests.CheckDelta(1)
	p.activeRequests.CheckDelta(0)
}

type fakePool struct {
	clientpool.Pool
}

func (fakePool) NumActiveClients() int32 {
	return 1
}

func (fakePool) NumAllocated() int32 {
	return 2
}

func TestClientPoolGaugeExporterRegister(t *testing.T) {
	// This test is to make sure that a service creating more than one thrift
	// client pool will not cause issues in prometheus metrics.
	exporters := []clientPoolGaugeExporter{
		{
			slug: "foo",
			pool: fakePool{},
		},
		{
			slug: "bar",
			pool: fakePool{},
		},
	}
	for i, exporter := range exporters {
		if err := prometheus.Register(exporter); err != nil {
			t.Errorf("Register #%d failed: %v", i, err)
		}
	}
}

func TestClientPoolGaugeExporterCollect(t *testing.T) {
	exporter := clientPoolGaugeExporter{
		slug: "slug",
		pool: fakePool{},
	}
	ch := make(chan prometheus.Metric)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Drain the channel
		for range ch {
		}
	}()
	t.Cleanup(func() {
		close(ch)
		wg.Wait()
	})
	// No real test here, we just want to make sure that Collect call will not
	// panic, which would happen if we have a label mismatch.
	exporter.Collect(ch)
}

// This is an thrift.TException implementation that does not wrap any error.
type customTException struct{}

func (customTException) TExceptionType() thrift.TExceptionType {
	return thrift.TExceptionTypeUnknown
}

func (customTException) Error() string {
	return "error"
}

var _ thrift.TException = customTException{}

func TestStringifyErrorType(t *testing.T) {
	for _, c := range []struct {
		label string
		want  string
		err   error
	}{
		{
			label: "nil",
			want:  "",
			err:   nil,
		},
		{
			label: "typed-nil",
			want:  "",
			err:   thrift.TException(nil),
		},
		{
			label: "plain",
			want:  "errors.errorString",
			err:   errors.New("foo"),
		},
		{
			label: "WrapTException",
			want:  "errors.errorString",
			err:   thrift.WrapTException(errors.New("foo")),
		},
		{
			label: "custom",
			want:  "thriftbp.customTException",
			err:   customTException{},
		},
		{
			label: "custom-pointer",
			want:  "thriftbp.customTException",
			err:   new(customTException),
		},
		{
			label: "baseplate.Error",
			want:  "baseplate.Error",
			err: &baseplate.Error{
				Code: thrift.Int32Ptr(10),
			},
		},
		{
			label: "TTransportException",
			want:  "thrift.tTransportException",
			err:   thrift.NewTTransportException(thrift.TIMED_OUT, "foo"),
		},
		{
			label: "TProtocolException",
			want:  "thrift.tProtocolException",
			err:   thrift.NewTProtocolException(errors.New("foo")),
		},
		{
			label: "TApplicationExceptino",
			want:  "thrift.tApplicationException",
			err:   thrift.NewTApplicationException(thrift.UNKNOWN_METHOD, "foo"),
		},
	} {
		t.Run(c.label, func(t *testing.T) {
			got := stringifyErrorType(c.err)
			if got != c.want {
				t.Errorf("want %q, got %q", c.want, got)
			}
		})
	}
}
