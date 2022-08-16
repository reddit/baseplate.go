package spectest

import (
	"errors"
	"reflect"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/reddit/baseplate.go/errorsbp"
	"github.com/reddit/baseplate.go/internalv2compat"
)

func TestMissingMetrics(t *testing.T) {
	testCases := []struct {
		name    string
		missing map[string]struct{}
		want    []string
	}{
		{
			name:    "none missing",
			missing: map[string]struct{}{},
		},
		{
			name:    "missing",
			missing: map[string]struct{}{"foo": {}},
			want:    []string{"foo"},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			if got, want := keysFrom(tt.missing), tt.want; !stringSlicesEqual(got, want) {
				t.Fatalf("got %v, want %v", got, want)
			}
		})
	}
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestBuildMetricsNames(t *testing.T) {
	testCases := []struct {
		name           string
		prefix         string
		clientOrServer string
		want           map[string]struct{}
	}{
		{
			name:           "with prefix",
			prefix:         "prefix",
			clientOrServer: "client",
			want: map[string]struct{}{
				"prefix_client_active_requests": {},
				"prefix_client_latency_seconds": {},
				"prefix_client_requests_total":  {},
			},
		},
		{
			name:           "without prefix",
			prefix:         "",
			clientOrServer: "server",
			want: map[string]struct{}{
				"server_active_requests": {},
				"server_latency_seconds": {},
				"server_requests_total":  {},
			},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			if got, want := buildMetricNames(tt.prefix, tt.clientOrServer), tt.want; !reflect.DeepEqual(got, want) {
				t.Fatalf("got %v, want %v", got, want)
			}
		})
	}
}

func TestValidateName(t *testing.T) {
	testCases := []struct {
		name         string
		metricName   string
		metricPrefix string
		wantErrs     []error
	}{
		{
			name:         "prefix not in beginning of metric name",
			metricName:   "foo_client_bar_total",
			metricPrefix: "bar",
			wantErrs:     []error{errPrefix},
		},
		{
			name:         "prefix not in beginning of metric name",
			metricName:   "foo_bar_total",
			metricPrefix: "fo",
			wantErrs:     []error{errPrefix, errClientServer},
		},
		{
			name:         "wrong length, only 1 part",
			metricName:   "foo",
			metricPrefix: "foo",
			wantErrs:     []error{errLength},
		},
		{
			name:         "wrong length, only 2 parts",
			metricName:   "foo_client",
			metricPrefix: "foo",
			wantErrs:     []error{errLength},
		},
		{
			name:         "wrong length, 0 parts",
			metricName:   "",
			metricPrefix: "foo",
			wantErrs:     []error{errLength},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			gotErr := validateName(tt.metricName, tt.metricPrefix, client)
			checkErrors(t, gotErr, tt.wantErrs)
		})
	}
}

func TestValidateLabels(t *testing.T) {
	testCases := []struct {
		name           string
		metricName     string
		prefix         string
		clientOrServer string
		gotLabels      map[string]struct{}
		wantErrs       []error
	}{
		{
			name:           "provide wrong labels",
			metricName:     "foo_bar_total",
			prefix:         "fo",
			clientOrServer: server,
			gotLabels: map[string]struct{}{
				"test": {},
			},
			wantErrs: []error{errDiffLabels},
		},
		{
			name:           "latency success",
			metricName:     "thrift_latency_seconds",
			prefix:         thriftPrefix,
			clientOrServer: client,
			gotLabels: map[string]struct{}{
				"thrift_method":  {},
				"thrift_success": {},
				"thrift_slug":    {},
			},
			wantErrs: []error{},
		},
		{
			name:           "latency wrong labels",
			metricName:     "thrift_latency_seconds",
			prefix:         thriftPrefix,
			clientOrServer: server,
			gotLabels: map[string]struct{}{
				"thrift_method": {},
			},
			wantErrs: []error{errDiffLabels},
		},
		{
			name:           "request total success",
			metricName:     "thrift_requests_total",
			prefix:         thriftPrefix,
			clientOrServer: server,
			gotLabels: map[string]struct{}{
				"thrift_method":                {},
				"thrift_success":               {},
				"thrift_baseplate_status":      {},
				"thrift_baseplate_status_code": {},
				"thrift_exception_type":        {},
				"baseplate_go":                 {},
			},
			wantErrs: []error{},
		},
		{
			name:           "request total labels",
			metricName:     "thrift_requests_total",
			prefix:         thriftPrefix,
			clientOrServer: server,
			gotLabels: map[string]struct{}{
				"thrift_method": {},
			},
			wantErrs: []error{errDiffLabels},
		},
		{
			name:           "active_requests success",
			metricName:     "thrift_active_requests",
			prefix:         thriftPrefix,
			clientOrServer: server,
			gotLabels: map[string]struct{}{
				"thrift_method": {},
			},
			wantErrs: []error{},
		},
		{
			name:           "active_requests wrong labels",
			metricName:     "thrift_active_requests",
			prefix:         thriftPrefix,
			clientOrServer: server,
			gotLabels: map[string]struct{}{
				"thrift_method": {},
				"foo":           {},
			},
			wantErrs: []error{errDiffLabels},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			tt.gotLabels["baseplate_go_is_v0"] = struct{}{} // set const label
			gotErr := validateLabels(tt.metricName, tt.prefix, tt.clientOrServer, tt.gotLabels)
			checkErrors(t, gotErr, tt.wantErrs)
		})
	}
}

func TestBuildLabelsThrift(t *testing.T) {
	testCases := []struct {
		name           string
		metricName     string
		prefix         string
		clientOrServer string
		want           map[string]struct{}
	}{
		{
			name:           "latency_seconds labels",
			metricName:     "thrift_latency_seconds",
			prefix:         thriftPrefix,
			clientOrServer: server,
			want: map[string]struct{}{
				"thrift_method":  {},
				"thrift_success": {},
			},
		},
		{
			name:           "requests_total labels",
			metricName:     "thrift_requests_total",
			prefix:         thriftPrefix,
			clientOrServer: client,
			want: map[string]struct{}{
				"thrift_method":                {},
				"thrift_success":               {},
				"thrift_baseplate_status":      {},
				"thrift_baseplate_status_code": {},
				"thrift_exception_type":        {},
				"thrift_slug":                  {},
			},
		},
		{
			name:           "active_requests labels",
			metricName:     "thrift_active_requests",
			prefix:         thriftPrefix,
			clientOrServer: server,
			want:           map[string]struct{}{"thrift_method": {}},
		},
		{
			name:           "none",
			prefix:         thriftPrefix,
			clientOrServer: server,
			want:           map[string]struct{}{},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			tt.want["baseplate_go_is_v0"] = struct{}{} // set const label
			if got, want := buildLabels(tt.metricName, tt.prefix, tt.clientOrServer), tt.want; !reflect.DeepEqual(got, want) {
				t.Fatalf("got %v, want %v", got, want)
			}
		})
	}
}

func TestBuildLabelsHTTP(t *testing.T) {
	testCases := []struct {
		name           string
		metricName     string
		prefix         string
		clientOrServer string
		want           map[string]struct{}
	}{
		{
			name:           "latency_seconds labels",
			metricName:     "http_latency_seconds",
			prefix:         httpPrefix,
			clientOrServer: server,
			want: map[string]struct{}{
				"http_method":   {},
				"http_success":  {},
				"http_endpoint": {},
			},
		},
		{
			name:           "http_server_response_size_bytes labels",
			metricName:     "http_server_response_size_bytes",
			prefix:         httpPrefix,
			clientOrServer: server,
			want: map[string]struct{}{
				"http_method":   {},
				"http_success":  {},
				"http_endpoint": {},
			},
		},
		{
			name:           "http_server_request_size_bytes labels",
			metricName:     "http_server_request_size_bytes",
			prefix:         httpPrefix,
			clientOrServer: server,
			want: map[string]struct{}{
				"http_method":   {},
				"http_success":  {},
				"http_endpoint": {},
			},
		},
		{
			name:           "http_client_request_size_bytes client labels",
			metricName:     "http_client_request_size_bytes",
			prefix:         httpPrefix,
			clientOrServer: client,
			want: map[string]struct{}{
				"http_slug": {},
			},
		},
		{
			name:           "requests_total labels",
			metricName:     "http_requests_total",
			prefix:         httpPrefix,
			clientOrServer: client,
			want: map[string]struct{}{
				"http_method":        {},
				"http_success":       {},
				"http_response_code": {},
				"http_slug":          {},
			},
		},
		{
			name:           "active_requests labels",
			metricName:     "http_active_requests",
			prefix:         httpPrefix,
			clientOrServer: server,
			want: map[string]struct{}{
				"http_method":   {},
				"http_endpoint": {},
			},
		},
		{
			name:           "none",
			prefix:         httpPrefix,
			clientOrServer: server,
			want:           map[string]struct{}{},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			tt.want["baseplate_go_is_v0"] = struct{}{} // set const label
			if got, want := buildLabels(tt.metricName, tt.prefix, tt.clientOrServer), tt.want; !reflect.DeepEqual(got, want) {
				t.Fatalf("got %v, want %v", got, want)
			}
		})
	}
}

func TestBuildLabelsGPRC(t *testing.T) {
	testCases := []struct {
		name           string
		metricName     string
		prefix         string
		clientOrServer string
		want           map[string]struct{}
	}{
		{
			name:           "latency_seconds labels",
			metricName:     "grpc_latency_seconds",
			prefix:         grpcPrefix,
			clientOrServer: server,
			want: map[string]struct{}{
				"grpc_service": {},
				"grpc_method":  {},
				"grpc_type":    {},
				"grpc_success": {},
			},
		},
		{
			name:           "requests_total labels",
			metricName:     "grpc_requests_total",
			prefix:         grpcPrefix,
			clientOrServer: client,
			want: map[string]struct{}{
				"grpc_service": {},
				"grpc_method":  {},
				"grpc_type":    {},
				"grpc_success": {},
				"grpc_code":    {},
				"grpc_slug":    {},
			},
		},
		{
			name:           "active_requests labels",
			metricName:     "grpc_active_requests",
			prefix:         grpcPrefix,
			clientOrServer: server,
			want: map[string]struct{}{
				"grpc_method":  {},
				"grpc_service": {},
			},
		},
		{
			name:           "none",
			prefix:         grpcPrefix,
			clientOrServer: server,
			want:           map[string]struct{}{},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			tt.want["baseplate_go_is_v0"] = struct{}{} // set const label
			if got, want := buildLabels(tt.metricName, tt.prefix, tt.clientOrServer), tt.want; !reflect.DeepEqual(got, want) {
				t.Fatalf("got %v, want %v", got, want)
			}
		})
	}
}

func TestValidateSpec(t *testing.T) {
	var (
		testLabels = []string{
			"thrift_method",
			"thrift_success",
			"thrift_baseplate_status",
		}

		testMetric = promauto.With(internalv2compat.GlobalRegistry).NewCounterVec(prometheus.CounterOpts{
			Name: "thrift_client_latency_seconds",
			Help: "Test help message",
		}, testLabels)
	)

	labels := prometheus.Labels{
		"thrift_method":           "foo",
		"thrift_success":          "foo",
		"thrift_baseplate_status": "foo",
	}
	testMetric.With(labels).Inc()

	testCases := []struct {
		name           string
		metric         prometheus.Collector
		prefix         string
		clientOrServer string
		wantCount      int
		wantErrs       []error
	}{
		{
			name:           "not found server",
			metric:         testMetric,
			prefix:         thriftPrefix,
			clientOrServer: "server",
			wantErrs:       []error{errNotFound},
		},
		{
			name:           "multi errs client",
			metric:         testMetric,
			prefix:         thriftPrefix,
			clientOrServer: "client",
			wantErrs:       []error{errPrometheusLint, errDiffLabels, errNotFound},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			gotErr := validateSpec(tt.prefix, tt.clientOrServer)
			checkErrors(t, gotErr, tt.wantErrs)
		})
	}
}

func checkErrors(tb testing.TB, gotErr error, wantErrs []error) {
	tb.Helper()
	tb.Logf("got errors %v, want errors %v", gotErr, wantErrs)

	if got, want := errorsbp.BatchSize(gotErr), len(wantErrs); got != want {
		tb.Errorf("not same number of errors got %d, want %d", got, want)
	}

	for _, wantErr := range wantErrs {
		if !errors.Is(gotErr, wantErr) {
			tb.Errorf("want error %v not in returned errors", wantErr)
		}
	}
}
