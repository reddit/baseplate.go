package runtimebp

import (
	"fmt"
)

// Config is the configuration struct for the runtimebp package.
//
// Can be parsed from YAML.
type Config struct {
	// NumProcesses can be used to set the maximum and minimum number of Go
	// Processes.
	NumProcesses struct {
		// Max controls the maximum number of Go processes.  This is the ceiling,
		// the final maximum number will depend on the number of CPUs available to
		// your service.
		//
		// Defaults to 64 if not set.
		Max int `yaml:"max"`

		// Max controls the minimum number of Go processes.
		//
		// Defaults to 1 if not set.
		Min int `yaml:"min"`
	} `yaml:"numProcesses"`
}

// InitFromConfig sets GOMAXPROCS using the given config.
func InitFromConfig(cfg Config) {
	max := 64
	min := 1
	if cfg.NumProcesses.Max != 0 {
		max = cfg.NumProcesses.Max
	}
	if cfg.NumProcesses.Min != 0 {
		min = cfg.NumProcesses.Min
	}
	prev, current := GOMAXPROCS(min, max)
	fmt.Println("GOMAXPROCS:", prev, current)
}
