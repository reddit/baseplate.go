package runtimebp

import (
	"fmt"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// NumCPU returns the number of CPUs assigned to this running container.
//
// This is the container aware version of runtime.NumCPU.
// It reads from the cgroup cpu.cfs_quota_us and cpu.cfs_period_us values
// to determine the hard CPU limit of the container.
//
// If the current process is not running inside a container,
// or if there's no limit set in cgroup,
// it will fallback to runtime.NumCPU() instead.
func NumCPU() float64 {
	const (
		quotaPath  = "/sys/fs/cgroup/cpu/cpu.cfs_quota_us"
		periodPath = "/sys/fs/cgroup/cpu/cpu.cfs_period_us"
	)

	// Default to the standard runtime package value.
	defaultCPUs := float64(runtime.NumCPU())

	// Big enough buffer to read the number in the file wholly into memory.
	buf := make([]byte, 1024)

	quota, err := readNumberFromFile(quotaPath, buf)
	if err != nil {
		fmt.Fprintf(
			os.Stderr,
			"NumCPU: Failed to read quota file, falling back to use runtime.NumCPU(): %v\n",
			err,
		)
		return defaultCPUs
	}

	// CFS quota returns -1 if there is no limit, return the default.
	if quota < 0 {
		fmt.Fprintf(
			os.Stderr,
			"NumCPU: Quota file returned %f, falling back to use runtime.NumCPU(): %v\n",
			quota,
			err,
		)
		return defaultCPUs
	}

	period, err := readNumberFromFile(periodPath, buf)
	if err != nil {
		fmt.Fprintf(
			os.Stderr,
			"NumCPU: Failed to read period file, falling back to use runtime.NumCPU(): %v\n",
			err,
		)
		return defaultCPUs
	}

	return quota / period
}

func readNumberFromFile(path string, buf []byte) (float64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("runtimebp: failed to open %q: %w", path, err)
	}
	defer file.Close()

	n, err := file.Read(buf)
	if err != nil {
		return 0, fmt.Errorf("runtimebp: failed to read %q: %w", path, err)
	}

	f, err := strconv.ParseInt(strings.TrimSpace(string(buf[:n])), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("runtimebp: failed to parse %q: %w", path, err)
	}
	return float64(f), nil
}

// MaxProcsFormula is the function to calculate GOMAXPROCS based on NumCPU value
// passed in.
type MaxProcsFormula func(n float64) int

func defaultMaxProcsFormula(n float64) int {
	return int(math.Ceil(n))
}

// GOMAXPROCS sets runtime.GOMAXPROCS with the default formula,
// in bound of [min, max].
//
// Currently the default formula is NumCPU() rounding up.
func GOMAXPROCS(min, max int) (oldVal, newVal int) {
	return GOMAXPROCSwithFormula(min, max, defaultMaxProcsFormula)
}

// GOMAXPROCSwithFormula sets runtime.GOMAXPROCS with the given formula,
// in bound of [min, max].
func GOMAXPROCSwithFormula(min, max int, formula MaxProcsFormula) (oldVal, newVal int) {
	newVal = boundNtoMinMax(formula(NumCPU()), min, max)
	oldVal = runtime.GOMAXPROCS(newVal)
	return
}

func boundNtoMinMax(n, min, max int) int {
	ret := float64(n)
	ret = math.Max(float64(min), ret)
	ret = math.Min(float64(max), ret)
	return int(ret)
}
