package runtimebp

import (
	"fmt"
	"math"
	"os"
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

// InitFromConfig sets GOMAXPROCS using the given config.
func InitFromConfig(cfg Config) {
	prev, current := GOMAXPROCS(1, math.MaxInt)
	fmt.Fprintf(os.Stderr, "GOMAXPROCS: Old: %d New: %d\n", prev, current)
}
