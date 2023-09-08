package spectest

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

const (
	client = "client"
	server = "server"

	thriftPrefix = "thrift"
	grpcPrefix   = "grpc"
	httpPrefix   = "http"

	partsCount = 3
)

var (
	errPrefix         = errors.New("the prefix is not at the beginning of the metric name")
	errClientServer   = fmt.Errorf("missing %q or %q as the second part of the metric name", client, server)
	errLength         = fmt.Errorf("metric name should have a minimum of %d parts", partsCount)
	errNotFound       = errors.New("metric not found")
	errPrometheusLint = errors.New("prometheus GatherAndLint")
	errDiffLabels     = errors.New("labels are incorrect")
)

// ValidateSpec validates that the Prometheus metrics being exposed from baseplate.go
// conform to the baseplate spec.
// metricPrefix should be either "grpc", "http", or "thrift".
// clientOrServer should be either "client" or "server".
func ValidateSpec(tb testing.TB, metricPrefix, clientOrServer string) {
	tb.Helper()

	if err := validateSpec(metricPrefix, clientOrServer); err != nil {
		tb.Errorf("validate spec %s: %v", clientOrServer, err)
	}
}

func validateSpec(prefix, clientOrServer string) error {
	metricFam, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		return err
	}

	metricsUnderTest := buildMetricNames(prefix, clientOrServer)

	var errs []error

	for _, m := range metricFam {
		name := *m.Name

		if _, ok := metricsUnderTest[name]; !ok {
			continue
		}

		errs = append(errs, lintMetric(name))
		errs = append(errs, validateName(name, prefix, clientOrServer))

		labels := make(map[string]struct{})
		for _, metric := range m.GetMetric() {
			for _, label := range metric.Label {
				labels[*label.Name] = struct{}{}
			}
		}
		errs = append(errs, validateLabels(name, prefix, clientOrServer, labels))

		// after we validate the metric, delete it from the set of all
		// expected metrics so that we can track if any were not found.
		delete(metricsUnderTest, name)
	}

	if len(metricsUnderTest) > 0 {
		errs = append(errs, fmt.Errorf("%w: %v", errNotFound, keysFrom(metricsUnderTest)))
	}

	return errors.Join(errs...)
}

func keysFrom(metrics map[string]struct{}) []string {
	missing := make([]string, 0, len(metrics))
	for k := range metrics {
		missing = append(missing, k)
	}
	return missing
}

// buildMetricNames creates a set of all the expected metrics names for the given prefix
// that should be registered with Prometheus as defined in the baseplate spec.
// The prefix will be either "grpc", "http", or "thrift".
func buildMetricNames(prefix, clientOrServer string) map[string]struct{} {
	suffixes := []string{"latency_seconds", "requests_total", "active_requests"}
	var names = map[string]struct{}{}
	for _, suffix := range suffixes {
		names[prometheus.BuildFQName(prefix, clientOrServer, suffix)] = struct{}{}
	}
	return names
}

// validateName checks the following:
//  1. the metric name has at least 3 parts separated by "_".
//     The parts are <namespace>_<client/server>_<name>_<suffix>, where suffix is optional.
//     Ref: https://prometheus.io/docs/practices/naming
//  2. the metric name has the correct prefix.
//  3. the metric contains either "client" or "server" as the second part.
func validateName(name, prefix, clientOrServer string) error {
	const separator = "_"

	parts := strings.SplitN(name, separator, partsCount)
	if len(parts) < partsCount {
		return fmt.Errorf("%w: name: %q part count: %d", errLength, name, len(parts))
	}

	var errs []error

	if got, want := parts[0], prefix; got != want {
		errs = append(errs, fmt.Errorf("%w: name: %q prefix: %q", errPrefix, name, prefix))
	}

	if got, want := parts[1], clientOrServer; got != want {
		errs = append(errs, fmt.Errorf("%w: name: %q", errClientServer, name))
	}
	return errors.Join(errs...)
}

func validateLabels(name, prefix, clientOrServer string, gotLabels map[string]struct{}) error {
	wantLabels := buildLabels(name, prefix, clientOrServer)
	if diff := cmp.Diff(gotLabels, wantLabels); diff != "" {
		return fmt.Errorf("%w: (-got +want)\n%s", errDiffLabels, diff)
	}
	return nil
}

// buildLabels returns a set of expected labels for the metric name provided.
// prefix is either thrift, http, or grpc.
func buildLabels(name, prefix, clientOrServer string) map[string]struct{} {
	var labelSuffixes []string
	switch prefix {
	case thriftPrefix:
		labelSuffixes = thriftSpecificLabels(name)
	case grpcPrefix:
		labelSuffixes = grpcSpecificLabels(name)
	case httpPrefix:
		labelSuffixes = httpSpecificLabels(name, clientOrServer)
	}

	if clientOrServer == client {
		labelSuffixes = append(labelSuffixes, "client_name")
	}

	wantLabels := make(map[string]struct{}, len(labelSuffixes))
	for _, label := range labelSuffixes {
		wantLabels[prefix+"_"+label] = struct{}{}
	}
	wantLabels["baseplate_go"] = struct{}{}
	return wantLabels
}

// thriftSpecificLabels returns the following labels:
// latency_seconds metrics expect the following labels:
//   - "method"
//   - "success"
//
// requests_total metrics expect the following labels:
//   - "method"
//   - "success"
//   - "exception_type"
//   - "baseplate_status"
//   - "baseplate_status_code"
//
// active_requests metrics expect the following labels:
//   - "method"
func thriftSpecificLabels(name string) []string {
	labelSuffixes := []string{"method"}
	switch {
	case strings.HasSuffix(name, "_latency_seconds"):
		labelSuffixes = append(labelSuffixes, "success")
	case strings.HasSuffix(name, "_requests_total"):
		labelSuffixes = append(labelSuffixes, "success", "exception_type", "baseplate_status", "baseplate_status_code")
	case strings.HasSuffix(name, "_active_requests"):
		// no op
	default:
		return nil
	}
	return labelSuffixes
}

// grpcSpecificLabels returns the following labels:
// latency_seconds metrics expect the following labels:
//   - "service"
//   - "method"
//   - "success"
//   - "type"
//
// requests_total metrics expect the following labels:
//   - "service"
//   - "method"
//   - "success"
//   - "type"
//   - "code"
//
// active_requests metrics expect the following labels:
//   - "service"
//   - "method"
func grpcSpecificLabels(name string) []string {
	labelSuffixes := []string{"service", "method", "type"}
	switch {
	case strings.HasSuffix(name, "_latency_seconds"):
		labelSuffixes = append(labelSuffixes, "success")
	case strings.HasSuffix(name, "_requests_total"):
		labelSuffixes = append(labelSuffixes, "success", "code")
	case strings.HasSuffix(name, "_active_requests"):
		// no op
	default:
		return nil
	}
	return labelSuffixes
}

// httpSpecificLabels returns the following labels:
// latency_seconds, server_request_size_bytes, server_response_size_bytes metrics expect the following labels:
//   - "method"
//   - "success"
//
// requests_total metrics expect the following labels:
//   - "method"
//   - "success"
//   - "response_code"
//
// active_requests metrics expect the following labels:
//   - "service"
//   - "method"
func httpSpecificLabels(name, clientOrServer string) []string {
	labelSuffixes := []string{"method", "endpoint"}

	switch {
	case strings.HasSuffix(name, "_latency_seconds"):
		labelSuffixes = append(labelSuffixes, "success")
	case strings.HasSuffix(name, "_server_request_size_bytes"):
		labelSuffixes = append(labelSuffixes, "success")
	case strings.HasSuffix(name, "_server_response_size_bytes"):
		labelSuffixes = append(labelSuffixes, "success")
	case strings.HasSuffix(name, "_requests_total"):
		labelSuffixes = append(labelSuffixes, "success", "response_code")
	case strings.HasSuffix(name, "_active_requests"):
		// no op
	default:
		return nil
	}
	return labelSuffixes
}

func lintMetric(metricName string) error {
	var errs []error
	problems, err := testutil.GatherAndLint(prometheus.DefaultGatherer, metricName)
	if err != nil {
		errs = append(errs, err)
	}
	for _, p := range problems {
		errs = append(errs, fmt.Errorf("%w: name: %q %s", errPrometheusLint, metricName, p.Text))
	}
	return errors.Join(errs...)
}
