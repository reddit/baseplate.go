package spectest

import (
	"errors"
	"fmt"
	"strings"
	"testing"

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
	errLabelNotFound  = errors.New("label not found")
	errPrometheusLint = errors.New("prometheus GatherAndLint")
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

	allMetricNames := buildMetricNames(prefix)

	for _, m := range metricFam {
		name := *m.Name

		if _, ok := allMetricNames[name]; !ok {
			continue
		}

		batch.Add(validatePromLint(name))
		batch.Add(validateName(name, prefix))

		labels := map[string]string{}
		for _, metric := range m.GetMetric() {
			for _, label := range metric.Label {
				labels[*label.Name] = *label.Value
			}
		}
		batch.Add(validateLabels(name, prefix, labels))

		// after we validate the metric, delete it from the set of all
		// expected metrics so that we can track if any were not found.
		delete(allMetricNames, name)
	}

	if len(allMetricNames) > 0 {
		notFound := make([]string, 0, len(allMetricNames))
		for k := range allMetricNames {
			notFound = append(notFound, k)
		}
		batch.Add(fmt.Errorf("%w metrics %v", errNotFound, notFound))
	}

	return batch
}

// buildMetricNames creates a set of all the expected metrics names for the given prefix
// that should be registered with Prometheus as defined in the baseplate spec.
// The prefix will be either thrift, http, or grpc.
func buildMetricNames(prefix string) map[string]struct{} {
	suffixes := []string{"_latency_seconds", "_requests_total", "_active_requests"}
	names := map[string]struct{}{}
	for _, suffix := range suffixes {
		names[prefix+"_"+server+suffix] = struct{}{}
		names[prefix+"_"+client+suffix] = struct{}{}
	}
	return names
}

// validateName checks the following:
// 1) the metric name has at least 3 parts separated by "_".
// The parts are <namespace>_<client/server>_<name>_<suffix>, where suffix is optional.
// Ref: https://prometheus.io/docs/practices/naming
// 2) the metric name has the correct prefix.
// 3) the metric contains either "client" or "server" as the second part.
func validateName(name, prefix string) error {
	var batch errorsbp.Batch

	const (
		separator = "_"
		partCount = 3
	)
	parts := strings.SplitN(name, separator, partCount)
	if len(parts) < partCount {
		batch.Add(fmt.Errorf("%w metric %q does not have minimum number (%d) of parts", errLength, name, partCount))
		return batch.Compile()
	}

	if got, want := parts[0], prefix; got != want {
		batch.Add(fmt.Errorf("%w metric %q does not have the expected prefix %q", errPrefix, name, prefix))
	}

	if parts[1] != client && parts[1] != server {
		batch.Add(fmt.Errorf("%w metric %q", errClientServer, name))
	}
	return batch.Compile()
}

func validateLabels(prefix, name string, labels map[string]string) error {
	var (
		localServiceLabel        = prefix + "_service"
		methodLabel              = prefix + "_method"
		successLabel             = prefix + "_success"
		exceptionLabel           = prefix + "_exception_type"
		baseplateStatusLabel     = prefix + "_baseplate_status"
		baseplateStatusCodeLabel = prefix + "_baseplate_status_code"
		remoteServiceSlugLabel   = prefix + "_slug"

		serverLatencyLabels = []string{
			localServiceLabel,
			methodLabel,
			successLabel,
		}

		serverRequestLabels = []string{
			localServiceLabel,
			methodLabel,
			successLabel,
			exceptionLabel,
			baseplateStatusLabel,
			baseplateStatusCodeLabel,
		}

		serverActiveRequestsLabels = []string{
			localServiceLabel,
			methodLabel,
		}

		clientLatencyLabels = []string{
			localServiceLabel,
			methodLabel,
			successLabel,
			remoteServiceSlugLabel,
		}
		clientRequestLabels = []string{
			localServiceLabel,
			methodLabel,
			successLabel,
			exceptionLabel,
			baseplateStatusLabel,
			baseplateStatusCodeLabel,
			remoteServiceSlugLabel,
		}
		clientActiveRequestsLabels = []string{
			localServiceLabel,
			methodLabel,
			remoteServiceSlugLabel,
		}
	)
	suffixes := []string{"_latency_seconds", "_requests_total", "_active_requests"}
	metricLabelPair := map[string][]string{}
	metricLabelPair[prefix+server+suffixes[0]] = serverLatencyLabels
	metricLabelPair[prefix+client+suffixes[0]] = clientLatencyLabels
	metricLabelPair[prefix+server+suffixes[1]] = serverRequestLabels
	metricLabelPair[prefix+client+suffixes[1]] = clientRequestLabels
	metricLabelPair[prefix+server+suffixes[2]] = serverActiveRequestsLabels
	metricLabelPair[prefix+client+suffixes[2]] = clientActiveRequestsLabels

	var batch errorsbp.Batch

	val, ok := metricLabelPair[name]
	if !ok {
		batch.AddPrefix("validate labels", fmt.Errorf("%w metric %q", errNotFound, name))
	}
	for _, wantLabel := range val {
		if _, ok := labels[wantLabel]; !ok {
			batch.Add(fmt.Errorf("%w metric %q label %q", errLabelNotFound, name, wantLabel))
		}
	}
	return batch.Compile()
}

func validatePromLint(metricName string) error {
	var batch errorsbp.Batch
	problems, err := testutil.GatherAndLint(prometheus.DefaultGatherer, metricName)
	if err != nil {
		batch.Add(err)
	}
	for _, p := range problems {
		batch.Add(fmt.Errorf("%w metric %q %s", errPrometheusLint, metricName, p.Text))
	}
	return batch.Compile()
}
