package spectest

import (
	"errors"
	"reflect"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
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
			want:    []string{},
		},
		{
			name:    "missing",
			missing: map[string]struct{}{"foo": {}},
			want:    []string{"foo"},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			if got, want := keysFrom(tt.missing), tt.want; !reflect.DeepEqual(got, want) {
				t.Fatalf("got %v, want %v", got, want)
			}
		})
	}
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
			gotBatch := validateName(tt.metricName, tt.metricPrefix)

			if got, want := len(gotBatch.GetErrors()), len(tt.wantErrs); got != want {
				t.Fatalf("not same number of errors got %d, want %d", got, want)
			}
			for i, e := range tt.wantErrs {
				if got, want := gotBatch.GetErrors()[i], e; !errors.Is(got, want) {
					t.Fatalf("not same errors got %d, want %d", got, want)
				}
			}
		})
	}
}

func TestValidateLabels(t *testing.T) {
	testCases := []struct {
		name       string
		metricName string
		prefix     string
		gotLabels  map[string]struct{}
		wantErrs   []error
	}{
		{
			name:       "provide wrong labels",
			metricName: "foo_bar_total",
			prefix:     "fo",
			gotLabels:  map[string]struct{}{"test": {}},
			wantErrs:   []error{errDiffLabels},
		},
		{
			name:       "latency success",
			metricName: "test_latency_seconds",
			prefix:     "test",
			gotLabels:  map[string]struct{}{"test_method": {}, "test_service": {}, "test_success": {}},
			wantErrs:   []error{},
		},
		{
			name:       "latency wrong labels",
			metricName: "test_latency_seconds",
			prefix:     "test",
			gotLabels:  map[string]struct{}{"test_method": {}},
			wantErrs:   []error{errDiffLabels},
		},
		{
			name:       "request total success",
			metricName: "test_requests_total",
			prefix:     "test",
			gotLabels: map[string]struct{}{
				"test_method":                {},
				"test_service":               {},
				"test_success":               {},
				"test_baseplate_status":      {},
				"test_baseplate_status_code": {},
				"test_exception_type":        {},
			},
			wantErrs: []error{},
		},
		{
			name:       "request total labels",
			metricName: "test_requests_total",
			prefix:     "test",
			gotLabels:  map[string]struct{}{"test_method": {}},
			wantErrs:   []error{errDiffLabels},
		},
		{
			name:       "active_requests success",
			metricName: "test_active_requests",
			prefix:     "test",
			gotLabels:  map[string]struct{}{"test_method": {}, "test_service": {}},
			wantErrs:   []error{},
		},
		{
			name:       "active_requests wrong labels",
			metricName: "test_active_requests",
			prefix:     "test",
			gotLabels:  map[string]struct{}{"test_method": {}},
			wantErrs:   []error{errDiffLabels},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			gotBatch := validateLabels(tt.metricName, tt.prefix, tt.gotLabels)

			if got, want := len(gotBatch.GetErrors()), len(tt.wantErrs); got != want {
				t.Fatalf("not same number of errors got %d, want %d", got, want)
			}
			for i, e := range tt.wantErrs {
				if got, want := gotBatch.GetErrors()[i], e; !errors.Is(got, want) {
					t.Fatalf("not same errors got %d, want %d", got, want)
				}
			}
		})
	}
}

func TestBuildLabels(t *testing.T) {
	testCases := []struct {
		name       string
		metricName string
		prefix     string
		want       map[string]struct{}
	}{
		{
			name:       "latency_seconds labels",
			metricName: "test_latency_seconds",
			prefix:     "test",
			want:       map[string]struct{}{"test_method": {}, "test_service": {}, "test_success": {}},
		},
		{
			name:       "requests_total labels",
			metricName: "test_requests_total",
			prefix:     "test",
			want: map[string]struct{}{
				"test_method":                {},
				"test_service":               {},
				"test_success":               {},
				"test_baseplate_status":      {},
				"test_baseplate_status_code": {},
				"test_exception_type":        {},
			},
		},
		{
			name:       "active_requests labels",
			metricName: "test_active_requests",
			prefix:     "test",
			want:       map[string]struct{}{"test_method": {}, "test_service": {}},
		},
		{
			name:   "none",
			prefix: "test",
			want:   map[string]struct{}{},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			if got, want := buildLables(tt.metricName, tt.prefix), tt.want; !reflect.DeepEqual(got, want) {
				t.Fatalf("got %v, want %v", got, want)
			}
		})
	}
}

func TestValidateSpec(t *testing.T) {
	var (
		testLabels = []string{
			"thrift_method",
			"thrift_service",
			"thrift_success",
			"thrift_baseplate_status",
		}

		multiErrsRequests = promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "thrift_client_latency_seconds",
			Help: "Test help message",
		}, testLabels)
	)

	labels := prometheus.Labels{
		"thrift_method":           "foo",
		"thrift_service":          "foo",
		"thrift_success":          "foo",
		"thrift_baseplate_status": "foo",
	}
	multiErrsRequests.With(labels).Inc()

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
			metric:         multiErrsRequests,
			prefix:         "thrift",
			clientOrServer: "server",
			wantErrs:       []error{errNotFound},
		},
		{
			name:           "multi errs client",
			metric:         multiErrsRequests,
			prefix:         "thrift",
			clientOrServer: "client",
			wantErrs:       []error{errPrometheusLint, errDiffLabels, errNotFound},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			gotBatch := validateSpec(tt.prefix, tt.clientOrServer)
			t.Log(gotBatch)

			if got, want := len(gotBatch.GetErrors()), len(tt.wantErrs); got != want {
				t.Fatalf("not same number of errors got %d, want %d", got, want)
			}
			for i, e := range tt.wantErrs {
				if got, want := gotBatch.GetErrors()[i], e; !errors.Is(got, want) {
					t.Fatalf("not same errors got %d, want %d", got, want)
				}
			}
		})
	}
}
