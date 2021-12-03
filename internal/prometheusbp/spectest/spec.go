package spectest

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/reddit/baseplate.go/errorsbp"
)

const (
	client = "client"
	server = "server"
)

var (
	errPrefix         = errors.New("the prefix is not at the beginning of the metric name")
	errClientServer   = fmt.Errorf("missing %q or %q as the second part of the metric name", client, server)
	errLength         = errors.New("metric name should have a minimum of three parts")
	errNotFound       = errors.New("metric not found")
	errPrometheusLint = errors.New("prometheus GatherAndLint")
	errDiffLabels     = errors.New("labels are incorrect")
)

// ValidateSpec validates that the Prometheus metrics being exposed from baseplate.go
// conform to the baseplate spec.
func ValidateSpec(t *testing.T, metricPrefix string, wantMetricCount int) {
	t.Helper()

	var batch errorsbp.Batch
	batch.AddPrefix("validate spec", validateSpec(metricPrefix))

	if len(batch.GetErrors()) > 0 {
		t.Errorf(batch.Compile().Error())
	}
}

func validateSpec(prefix string) errorsbp.Batch {
	var batch errorsbp.Batch

	metricFam, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		return batch
	}

	metricsUnderTest := buildMetricNames(prefix)

	for _, m := range metricFam {
		name := *m.Name

		if _, ok := metricsUnderTest[name]; !ok {
			continue
		}

		batch.Add(validatePromLint(name))
		batch.Add(validateName(name, prefix))

		labels := map[string]struct{}{}
		for _, metric := range m.GetMetric() {
			for _, label := range metric.Label {
				labels[*label.Name] = struct{}{}
			}
		}
		batch.Add(validateLabels(name, prefix, labels))

		// after we validate the metric, delete it from the set of all
		// expected metrics so that we can track if any were not found.
		delete(metricsUnderTest, name)
	}

	batch.Add(fmt.Errorf("%w %v", errNotFound, missingMetrics(metricsUnderTest)))

	return batch
}

func missingMetrics(metrics map[string]struct{}) []string {
	missing := []string{}
	if len(metrics) > 0 {
		for m := range metrics {
			missing = append(missing, m)
		}
	}
	return missing
}

// buildMetricNames creates a set of all the expected metrics names for the given prefix
// that should be registered with Prometheus as defined in the baseplate spec.
// The prefix will be either thrift, http, or grpc.
func buildMetricNames(prefix string) map[string]struct{} {
	suffixes := []string{"latency_seconds", "requests_total", "active_requests"}
	names := map[string]struct{}{}
	for _, suffix := range suffixes {
		names[prometheus.BuildFQName(prefix, server, suffix)] = struct{}{}
		names[prometheus.BuildFQName(prefix, client, suffix)] = struct{}{}
	}
	return names
}

// validateName checks the following:
// 1) the metric name has at least 3 parts separated by "_".
// The parts are <namespace>_<client/server>_<name>_<suffix>, where suffix is optional.
// Ref: https://prometheus.io/docs/practices/naming
// 2) the metric name has the correct prefix.
// 3) the metric contains either "client" or "server" as the second part.
func validateName(name, prefix string) errorsbp.Batch {
	var batch errorsbp.Batch

	const (
		separator = "_"
		partCount = 3
	)
	parts := strings.SplitN(name, separator, partCount)
	if len(parts) < partCount {
		batch.Add(fmt.Errorf("%w metric %q does not have minimum number (%d) of parts", errLength, name, partCount))
		return batch
	}

	if got, want := parts[0], prefix; got != want {
		batch.Add(fmt.Errorf("%w metric %q does not have the expected prefix %q", errPrefix, name, prefix))
	}

	if parts[1] != client && parts[1] != server {
		batch.Add(fmt.Errorf("%w metric %q", errClientServer, name))
	}
	return batch
}

func validateLabels(name, prefix string, gotLabels map[string]struct{}) errorsbp.Batch {
	var batch errorsbp.Batch

	wantLabels := buildLables(name, prefix)
	if diff := cmp.Diff(gotLabels, wantLabels); diff != "" {
		batch.Add(fmt.Errorf("%w: (-got +want)\n%s", errDiffLabels, diff))
	}

	return batch
}

// buildLables returns a set of expected labels for the metric name provided.
// prefix is either thrift, http, or grpc.
// latency_seconds metrics expect the following labels:
//   - "<prefix>_service"
//   - "<prefix>_method"
//   - "<prefix>_success"
// request_total metrics expect the following labels:
//   - "<prefix>_service"
//   - "<prefix>_method"
//   - "<prefix>_success"
//   - "<prefix>_exception_type"
//   - "<prefix>_baseplate_status"
//   - "<prefix>_baseplate_status_code"
// active_requests metrics expect the following labels:
//   - "<prefix>_service"
//   - "<prefix>_method"
func buildLables(name, prefix string) map[string]struct{} {
	labelSuffixes := []string{"service", "method"}
	successLabelSuffix := "success"
	switch {
	case strings.HasSuffix(name, "_latency_seconds"):
		labelSuffixes = append(labelSuffixes, successLabelSuffix)
	case strings.HasSuffix(name, "_requests_total"):
		labelSuffixes = append(labelSuffixes, successLabelSuffix, "exception_type", "baseplate_status", "baseplate_status_code")
	case strings.HasSuffix(name, "_active_requests"):
		// no op
	default:
		labelSuffixes = []string{}
	}
	var wantLabels = map[string]struct{}{}
	for _, label := range labelSuffixes {
		wantLabels[prefix+"_"+label] = struct{}{}
	}
	return wantLabels
}

func validatePromLint(metricName string) errorsbp.Batch {
	var batch errorsbp.Batch
	problems, err := testutil.GatherAndLint(prometheus.DefaultGatherer, metricName)
	if err != nil {
		batch.Add(err)
	}
	for _, p := range problems {
		batch.Add(fmt.Errorf("%w metric %q %s", errPrometheusLint, metricName, p.Text))
	}
	return batch
}
