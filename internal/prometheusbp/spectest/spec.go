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
// metricPrefix should be either "grpc", "http", or "thirft".
// clientOrServer should be either "client" or "server".
func ValidateSpec(tb testing.TB, metricPrefix, clientOrServer string) {
	tb.Helper()

	var batch errorsbp.Batch
	batch.AddPrefix("validate spec", validateSpec(metricPrefix, clientOrServer))

	if err := batch.Compile(); err != nil {
		tb.Error(err)
	}
}

func validateSpec(prefix, clientOrServer string) error {
	var batch errorsbp.Batch

	metricFam, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		batch.Add(err)
		return batch.Compile()
	}

	metricsUnderTest := buildMetricNames(prefix, clientOrServer)

	for _, m := range metricFam {
		name := *m.Name

		if _, ok := metricsUnderTest[name]; !ok {
			continue
		}

		batch.Add(lintMetric(name))
		batch.Add(validateName(name, prefix, clientOrServer))

		labels := map[string]struct{}{}
		for _, metric := range m.GetMetric() {
			for _, label := range metric.Label {
				labels[*label.Name] = struct{}{}
			}
		}
		batch.Add(validateLabels(name, prefix, clientOrServer, labels))

		// after we validate the metric, delete it from the set of all
		// expected metrics so that we can track if any were not found.
		delete(metricsUnderTest, name)
	}

	if len(metricsUnderTest) > 0 {
		batch.Add(fmt.Errorf("%w: %v", errNotFound, keysFrom(metricsUnderTest)))
	}

	return batch.Compile()
}

func keysFrom(metrics map[string]struct{}) []string {
	missing := []string{}
	if len(metrics) > 0 {
		for k := range metrics {
			missing = append(missing, k)
		}
	}
	return missing
}

// buildMetricNames creates a set of all the expected metrics names for the given prefix
// that should be registered with Prometheus as defined in the baseplate spec.
// The prefix will be either thrift, http, or grpc.
func buildMetricNames(prefix, clientOrServer string) map[string]struct{} {
	suffixes := []string{"latency_seconds", "requests_total", "active_requests"}
	names := map[string]struct{}{}
	for _, suffix := range suffixes {
		names[prometheus.BuildFQName(prefix, clientOrServer, suffix)] = struct{}{}
	}
	return names
}

// validateName checks the following:
//   1) the metric name has at least 3 parts separated by "_".
//     The parts are <namespace>_<client/server>_<name>_<suffix>, where suffix is optional.
//     Ref: https://prometheus.io/docs/practices/naming
//   2) the metric name has the correct prefix.
//   3) the metric contains either "client" or "server" as the second part.
func validateName(name, prefix, clientOrServer string) error {
	var batch errorsbp.Batch

	const (
		separator = "_"
		partCount = 3
	)
	parts := strings.SplitN(name, separator, partCount)
	if len(parts) < partCount {
		batch.Add(fmt.Errorf("%w: name: %q part count: %d", errLength, name, len(parts)))
		return batch.Compile()
	}

	if got, want := parts[0], prefix; got != want {
		batch.Add(fmt.Errorf("%w: name: %q prefix: %q", errPrefix, name, prefix))
	}

	if got, want := parts[1], clientOrServer; got != want {
		batch.Add(fmt.Errorf("%w: name: %q", errClientServer, name))
	}
	return batch.Compile()
}

func validateLabels(name, prefix, clientOrServer string, gotLabels map[string]struct{}) error {
	wantLabels := buildLables(name, prefix, clientOrServer)
	if diff := cmp.Diff(gotLabels, wantLabels); diff != "" {
		return fmt.Errorf("%w: (-got +want)\n%s", errDiffLabels, diff)
	}
	return nil
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
func buildLables(name, prefix, clientOrServer string) map[string]struct{} {
	labelSuffixes := []string{"service", "method"}
	switch {
	case strings.HasSuffix(name, "_latency_seconds"):
		labelSuffixes = append(labelSuffixes, "success")
	case strings.HasSuffix(name, "_requests_total"):
		labelSuffixes = append(labelSuffixes, "success", "exception_type", "baseplate_status", "baseplate_status_code")
	case strings.HasSuffix(name, "_active_requests"):
		// no op
	default:
		labelSuffixes = []string{}
	}

	if clientOrServer == client {
		labelSuffixes = append(labelSuffixes, "slug")
	}

	var wantLabels = map[string]struct{}{}
	for _, label := range labelSuffixes {
		wantLabels[prefix+"_"+label] = struct{}{}
	}
	return wantLabels
}

func lintMetric(metricName string) error {
	var batch errorsbp.Batch
	problems, err := testutil.GatherAndLint(prometheus.DefaultGatherer, metricName)
	if err != nil {
		batch.Add(err)
	}
	for _, p := range problems {
		batch.Add(fmt.Errorf("%w: name: %q %s", errPrometheusLint, metricName, p.Text))
	}
	return batch.Compile()
}
