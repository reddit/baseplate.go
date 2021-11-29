package promtest

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

var (
	errPrefix         = errors.New("the prefix is not at the beginning of the metric name")
	errLength         = errors.New("metric name should have a minimum of 3 parts, like prefix_name_suffix")
	errCount          = errors.New("wrong metric count for prefix")
	errPrometheusLint = errors.New("problem with Prometheus GatherAndLint")
)

// ValidateSpec validates that the Prometheus metrics being exposed conform to
// the baseplate spec.
func ValidateSpec(t *testing.T, metricPrefix string, wantMetricCount int) {
	if err := validateSpec(metricPrefix, wantMetricCount); err != nil {
		t.Error(err)
	}
}

func validateSpec(metricPrefix string, wantMetricCount int) error {
	metricFam, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		return err
	}

	var gotMetricCount int
	for _, m := range metricFam {
		metricName := *m.Name

		if strings.Contains(metricName, metricPrefix) {
			gotMetricCount++
			if err := validatePromLint(metricName); err != nil {
				return err
			}
			if err := validateName(metricName, metricPrefix); err != nil {
				return err
			}
		}
	}

	if gotMetricCount != wantMetricCount {
		return fmt.Errorf("%w: got %d, want %d", errCount, gotMetricCount, wantMetricCount)
	}
	return nil
}

// validateName checks that:
// 1) the metric name has at least 3 parts separated by "_".
// The parts are <namespace>_<metric>_<suffix> as described in Prometheus naming conventions.
// Ref: https://prometheus.io/docs/practices/naming
// 2) the metric name has the correct prefix.
func validateName(name, prefix string) error {
	metricNameParts := strings.Split(name, "_")
	if len(metricNameParts) < 3 {
		return fmt.Errorf("%w: got %d, want > 3", errLength, len(metricNameParts))
	}
	if !strings.HasPrefix(name, prefix+"_") {
		return fmt.Errorf("%w: got %s, want prefix %s", errPrefix, name, prefix)
	}
	return nil
}

func validatePromLint(metricName string) error {
	problems, err := testutil.GatherAndLint(prometheus.DefaultGatherer, metricName)
	if err != nil {
		return err
	}
	for _, p := range problems {
		return fmt.Errorf("%w: metric %s, problem %s", errPrometheusLint, metricName, p.Text)
	}
	return nil
}
