package spectest

import (
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	testLabels = []string{
		"testlabel",
	}

	suffixRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "suffix_client_test_foo",
		Help: "Test help message",
	}, testLabels)

	multiErrsRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "bar_baz",
		Help: "Test help message",
	}, testLabels)
)

func TestValidateSpec(t *testing.T) {
	labels := prometheus.Labels{
		"testlabel": "foo",
	}
	suffixRequests.With(labels).Inc()
	multiErrsRequests.With(labels).Inc()

	testCases := []struct {
		name      string
		metric    prometheus.Collector
		prefix    string
		wantCount int
		wantErrs  []error
	}{
		{
			name:      "wrong suffix",
			metric:    suffixRequests,
			prefix:    "suffix",
			wantCount: 1,
			wantErrs:  []error{errPrometheusLint},
		},
		{
			name:      "multi errs",
			metric:    multiErrsRequests,
			prefix:    "bar",
			wantCount: 1,
			wantErrs:  []error{errPrometheusLint, errLength},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			gotCount, gotBatch := validateSpec(tt.prefix)
			t.Log(gotBatch)

			if got, want := len(gotBatch.GetErrors()), len(tt.wantErrs); got != want {
				t.Fatalf("not same number of errors got %d, want %d", got, want)
			}
			for i, e := range tt.wantErrs {
				if got, want := gotBatch.GetErrors()[i], e; !errors.Is(got, want) {
					t.Fatalf("not same errors got %d, want %d", got, want)
				}
			}

			if gotCount != tt.wantCount {
				t.Fatalf("metric count err got: %v, want: %v", gotCount, tt.wantCount)
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
			gotErrs := validateName(tt.metricName, tt.metricPrefix)
			if len(gotErrs) != len(tt.wantErrs) {
				t.Fatalf("wrong number of errs got: %d %v, want: %d %v", len(gotErrs), gotErrs, len(tt.wantErrs), tt.wantErrs)
			}
			if len(gotErrs) > 0 {
				for i, e := range gotErrs {
					if errors.Unwrap(e) != tt.wantErrs[i] {
						t.Fatalf("got: %v, want: %v", e, tt.wantErrs[i])
					}
				}
			}
		})
	}
}
