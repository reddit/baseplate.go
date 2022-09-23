package admin

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

func TestMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector(baseplateGoCollectors))

	result, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}

	got := []string{}
	for _, r := range result {
		got = append(got, r.GetName())
	}

	if diff := cmp.Diff(expectedMetrics, got); diff != "" {
		t.Errorf("registered metrics mismatch (-want +got):\n%s", diff)
	}
}
