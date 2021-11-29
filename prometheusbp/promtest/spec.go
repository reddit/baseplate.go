package promtest

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/reddit/baseplate.go/errorsbp"
)

var (
	errPrefix         = errors.New("the prefix is not at the beginning of the metric name")
	errLength         = errors.New("metric name should have a minimum of 3 parts, like prefix_name_suffix")
	errCount          = errors.New("wrong metric count for prefix")
	errPrometheusLint = errors.New("problem with Prometheus GatherAndLint")
)

// ValidateSpec validates that the Prometheus metrics being exposed conform to
// the baseplate spec. This test will validate all metrics that begin with the
// metricPrefix and will validate the number of metrics with that prefix match
// wantMetricCount.
func ValidateSpec(t *testing.T, metricPrefix string, wantMetricCount int) {
	var batchErrs errorsbp.Batch
	gotMetricCount, errs := validateSpec(metricPrefix)
	if len(errs) > 0 {
		batchErrs.Add(errs...)
	}
	if gotMetricCount != wantMetricCount {
		batchErrs.Add(fmt.Errorf("%w: got %d, want %d", errCount, gotMetricCount, wantMetricCount))
	}
	if len(batchErrs.GetErrors()) > 0 {
		t.Error(batchErrs.GetErrors())
	}
}

func validateSpec(metricPrefix string) (int, []error) {
	var metricCount int
	var batchErrs errorsbp.Batch

	metricFam, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		return metricCount, batchErrs.GetErrors()
	}

	for _, m := range metricFam {
		metricName := *m.Name

		if !strings.HasPrefix(metricName, metricPrefix) {
			continue
		}

		metricCount++
		if errs := validatePromLint(metricName); len(errs) > 0 {
			batchErrs.Add(errs...)
		}
		if errs := validateName(metricName, metricPrefix); len(errs) > 0 {
			batchErrs.Add(errs...)
		}
	}

	return metricCount, batchErrs.GetErrors()
}

// validateName checks that:
// 1) the metric name has at least 3 parts separated by "_".
// The parts are <namespace>_<metric>_<suffix> as described in Prometheus naming conventions.
// Ref: https://prometheus.io/docs/practices/naming
// 2) the metric name has the correct prefix.
func validateName(name, prefix string) []error {
	const metricPartSeparator = "_"
	var batchErrs errorsbp.Batch
	metricNameParts := strings.Split(name, metricPartSeparator)
	if len(metricNameParts) < 3 {
		batchErrs.Add(fmt.Errorf("%w: got %d, want > 3", errLength, len(metricNameParts)))
	}
	if !strings.HasPrefix(name, prefix+metricPartSeparator) {
		batchErrs.Add(fmt.Errorf("%w: got %s, want prefix %s", errPrefix, name, prefix+metricPartSeparator))
	}
	return batchErrs.GetErrors()
}

func validatePromLint(metricName string) []error {
	var batchErrs errorsbp.Batch
	problems, err := testutil.GatherAndLint(prometheus.DefaultGatherer, metricName)
	if err != nil {
		batchErrs.Add(err)
	}
	for _, p := range problems {
		batchErrs.Add(fmt.Errorf("%w: metric %s, problem %s", errPrometheusLint, metricName, p.Text))
	}
	return batchErrs.GetErrors()
}
