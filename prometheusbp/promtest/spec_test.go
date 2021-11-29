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
		Name: "suffix_test_boop",
		Help: "Test help message",
	}, testLabels)

	countRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "count_test_total",
		Help: "Test help message",
	}, testLabels)
)

func TestValidateSpec(t *testing.T) {
	labels := prometheus.Labels{
		"testlabel": "foo",
	}
	suffixRequests.With(labels).Inc()
	countRequests.With(labels).Inc()

	testCases := []struct {
		name    string
		metric  prometheus.Collector
		prefix  string
		count   int
		wantErr error
	}{
		{
			name:    "wrong suffix",
			metric:  suffixRequests,
			prefix:  "suffix",
			count:   1,
			wantErr: errPrometheusLint,
		},
		{
			name:    "wrong metric count",
			metric:  countRequests,
			prefix:  "count",
			count:   10,
			wantErr: errCount,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			if gotErr := validateSpec(tt.prefix, tt.count); errors.Unwrap(gotErr) != tt.wantErr {
				t.Fatalf("got: %v, want: %v", gotErr, tt.wantErr)
			}
		})
	}
}

func TestValidateName(t *testing.T) {
	testCases := []struct {
		name         string
		metricName   string
		metricPrefix string
		wantErr      error
	}{
		{
			name:         "prefix not in beginning of metric name",
			metricName:   "foo_bar_total",
			metricPrefix: "bar",
			wantErr:      errPrefix,
		},
		{
			name:         "prefix not in beginning of metric name",
			metricName:   "foo_bar_total",
			metricPrefix: "fo",
			wantErr:      errPrefix,
		},
		{
			name:         "wrong length, only 1 part",
			metricName:   "foo",
			metricPrefix: "foo",
			wantErr:      errLength,
		},
		{
			name:         "wrong length, only 2 parts",
			metricName:   "foo_total",
			metricPrefix: "foo",
			wantErr:      errLength,
		},
		{
			name:         "wrong length, 0 parts",
			metricName:   "",
			metricPrefix: "foo",
			wantErr:      errLength,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			gotErr := validateName(tt.metricName, tt.metricPrefix)

			if errors.Unwrap(gotErr) != tt.wantErr {
				t.Fatalf("got: %v, want: %v", gotErr, tt.wantErr)
			}
		})
	}
}
