package prometheusbp

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
		}, {
			Path: "github.com/reddit/oldmodule",
			Replace: &debug.Module{
				Path:    "github.com/reddit/newmodule",
				Version: "v1.42.0",
			},
			Version: "v0.1.2",
		}},
	}

	defer promtest.NewPrometheusMetricTest(t, "baseplate_go_modules", goModules, prometheus.Labels{
		"go_module":       "example.com/path/to/main/module",
		"module_role":     "main",
		"module_replaced": "false",
		"module_version":  "(devel)",
	}).CheckDelta(1)

	defer promtest.NewPrometheusMetricTest(t, "baseplate_go_modules", goModules, prometheus.Labels{
		"go_module":       "github.com/reddit/baseplate.go",
		"module_role":     "dependency",
		"module_replaced": "false",
		"module_version":  "v1.2.3",
	}).CheckDelta(1)

	defer promtest.NewPrometheusMetricTest(t, "baseplate_go_modules", goModules, prometheus.Labels{
		"go_module":       "github.com/reddit/oldmodule",
		"module_role":     "dependency",
		"module_replaced": "true",
		"module_version":  "v0.1.2",
	}).CheckDelta(1)

	RecordModuleVersions(info)
}
