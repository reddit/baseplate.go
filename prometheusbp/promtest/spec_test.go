package promtest

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
		Name: "suffix_test_foo",
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
			gotCount, gotErrs := validateSpec(tt.prefix)
			if len(gotErrs) != len(tt.wantErrs) {
				t.Fatalf("wrong number of errs got: %d, want: %d", len(gotErrs), len(tt.wantErrs))
			}

			if len(gotErrs) > 0 {
				for i, e := range gotErrs {
					if errors.Unwrap(e) != tt.wantErrs[i] {
						t.Fatalf("got: %v, want: %v", e, tt.wantErrs[i])
					}
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
			metricName:   "foo_bar_total",
			metricPrefix: "bar",
			wantErrs:     []error{errPrefix},
		},
		{
			name:         "prefix not in beginning of metric name",
			metricName:   "foo_bar_total",
			metricPrefix: "fo",
			wantErrs:     []error{errPrefix},
		},
		{
			name:         "wrong length, only 1 part",
			metricName:   "foo",
			metricPrefix: "foo",
			wantErrs:     []error{errLength, errPrefix},
		},
		{
			name:         "wrong length, only 2 parts",
			metricName:   "foo_total",
			metricPrefix: "foo",
			wantErrs:     []error{errLength},
		},
		{
			name:         "wrong length, 0 parts",
			metricName:   "",
			metricPrefix: "foo",
			wantErrs:     []error{errLength, errPrefix},
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
