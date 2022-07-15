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
//
// NOTE: If the GOMAXPROCS environment variable is set,
// this function will skip setting GOMAXPROCS,
// even if the environment variable is set to some bogus value
// (in that case it will be set to number of physical CPUs).
func InitFromConfig(cfg Config) {
	if v, ok := os.LookupEnv("GOMAXPROCS"); ok {
		fmt.Fprintf(
			os.Stderr,
			"runtimebp.InitFromConfig: GOMAXPROCS environment variable is set to %q, skipping setting GOMAXPROCS.",
			v,
		)
	} else {
		prev, current := GOMAXPROCS(defaultMultiplier, math.MaxInt)
		fmt.Fprintf(os.Stderr, "GOMAXPROCS: Old: %d New: %d\n", prev, current)
	}
}
