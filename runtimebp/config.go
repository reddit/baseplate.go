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

// InitFromConfig sets GOMAXPROCS based on an overridable heuristic described in maxprocs.
//
// NOTE: It does NOT respect the passed-in config.
func InitFromConfig(_ Config) {
	maxprocs.Set()
}
