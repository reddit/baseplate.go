package prometheusbp

import (
	"runtime/debug"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/reddit/baseplate.go/internal/prometheusbpint"
)

var goModules = promauto.With(prometheusbpint.GlobalRegistry).NewGaugeVec(prometheus.GaugeOpts{
	Name: "baseplate_go_modules",
	Help: "Export the version information for included Go modules, and whether the module is the 'main' module or a 'dependency'.  Always 1",
}, []string{"go_module", "module_role", "module_replaced", "module_version"})

// RecordModuleVersions records the modules linked into this binary in the
// baseplate_go_modules prometheus metric.
//
// Users should not need to call this directly, as it is called by baseplate.New.
// This should generally not be called more than once, and it is not safe to call concurrently.
func RecordModuleVersions(info *debug.BuildInfo) {
	record := func(role string, mod *debug.Module) {
		goModules.With(prometheus.Labels{
			"go_module":       mod.Path,
			"module_role":     role,
			"module_replaced": strconv.FormatBool(mod.Replace != nil),
			"module_version":  mod.Version,
		}).Set(1)
	}

	goModules.Reset()
	record("main", &info.Main)
	for _, dep := range info.Deps {
		record("dependency", dep)
	}
}
