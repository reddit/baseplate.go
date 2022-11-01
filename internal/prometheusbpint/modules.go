package prometheusbpint

import (
	"runtime/debug"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var goModules = promauto.With(GlobalRegistry).NewGaugeVec(prometheus.GaugeOpts{
	Name: "baseplate_go_modules",
	Help: "Export the version information for included Go modules, and whether the module is the 'main' module or a 'dependency'.  Always 1",
}, []string{"go_module", "module_role", "replaced", "module_version"})

func RecordModuleVersions(info *debug.BuildInfo) {
	record := func(role string, mod *debug.Module) {
		goModules.With(prometheus.Labels{
			"go_module":      mod.Path,
			"module_role":    role,
			"replaced":       strconv.FormatBool(mod.Replace != nil),
			"module_version": mod.Version,
		}).Set(1)
	}

	goModules.Reset()
	record("main", &info.Main)
	for _, dep := range info.Deps {
		record("dependency", dep)
	}
}
