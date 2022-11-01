package prometheusbpint

import (
	"runtime/debug"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/reddit/baseplate.go/prometheusbp/promtest"
)

func TestBuildInfoMetrics(t *testing.T) {
	info := &debug.BuildInfo{
		GoVersion: "go1.18.3",
		Path:      "example.com/path/to/main/module",
		Main: debug.Module{
			Path:    "example.com/path/to/main/module",
			Version: "(devel)",
		},
		Deps: []*debug.Module{{
			Path:    "github.com/reddit/baseplate.go",
			Version: "v1.2.3",
		}},
	}

	defer promtest.NewPrometheusMetricTest(t, "baseplate_go_modules", goModules, prometheus.Labels{
		"go_module":      "example.com/path/to/main/module",
		"module_role":    "main",
		"replaced":       "false",
		"module_version": "(devel)",
	}).CheckDelta(1)

	defer promtest.NewPrometheusMetricTest(t, "baseplate_go_modules", goModules, prometheus.Labels{
		"go_module":      "github.com/reddit/baseplate.go",
		"module_role":    "dependency",
		"replaced":       "false",
		"module_version": "v1.2.3",
	}).CheckDelta(1)

	RecordModuleVersions(info)
}
