package runtimebp

import (
	"github.com/reddit/baseplate.go/runtimebp/internal/maxprocs"
)

// Config is the configuration struct for the runtimebp package.
//
// Can be parsed from YAML.
type Config struct {
	// Deprecated: No-op for now, will be removed in a future release.
	NumProcesses struct {
		// Deprecated: Always overridden to math.MaxInt.
		Max int `yaml:"max"`

		// Deprecated: Always overridden to 1.
		Min int `yaml:"min"`
	} `yaml:"numProcesses"`
}

// InitFromConfig configures the runtime's GOMAXPROCS using the following heuristic:
//   1. If $GOMAXPROCS is set, it relinquishes control to the Go runtime.
//      This should cause the runtime to respect this value directly.
//   2. If $BASEPLATE_CPU_REQUEST is unset/invalid, it relinquishes control to automaxprocs, minimum 2.
//      See https://pkg.go.dev/go.uber.org/automaxprocs for specific behavior.
//   3. Otherwise, $BASEPLATE_CPU_REQUEST is multiplied by $BASEPLATE_CPU_REQUEST_SCALE
//      (or defaultCPURequestScale) to compute the new GOMAXPROCS, minimum 2.
//
// InitFromConfig also exports several metrics to facilitate further tuning/analysis.
//
// NOTE: It does NOT respect the passed-in config.
func InitFromConfig(_ Config) {
	maxprocs.Set()
}
