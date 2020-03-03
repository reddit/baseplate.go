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
// It reads from the cgroup sysfs values.
//
// If the current process is not running inside a container,
// or for whatever reason we failed to read the cgroup sysfs values,
// it will fallback to runtime.NumCPU() instead.
//
// When fallback happens, it also prints the reason to stderr.
func NumCPU() (n float64) {
	const (
		quotaPath  = "/sys/fs/cgroup/cpu/cpu.cfs_quota_us"
		periodPath = "/sys/fs/cgroup/cpu/cpu.cfs_period_us"
	)

	var err error
	defer func() {
		if err != nil || n <= 0 {
			// Fallback and log to stderr.
			fmt.Fprintf(
				os.Stderr,
				"NumCPU: falling back to use shares: %v, %v\n",
				n,
				err,
			)
			n = numCPUSharesFallback()
		}
	}()

	// Big enough buffer to read the number in the file wholly into memory.
	buf := make([]byte, 1024)
	var quota, period float64

	quota, err = readNumberFromFile(quotaPath, buf)
	if err != nil {
		return
	}

	period, err = readNumberFromFile(periodPath, buf)
	if err != nil {
		return
	}

	return quota / period
}

// On some really old docker version the quota file will be -1, in which case we
// should use this one instead.
func numCPUSharesFallback() (n float64) {
	const (
		sharesPath  = "/sys/fs/cgroup/cpu/cpu.shares"
		denominator = 1024
	)

	var err error
	defer func() {
		if err != nil || n <= 0 {
			// Fallback and log to stderr.
			fmt.Fprintf(
				os.Stderr,
				"NumCPU: falling back to use runtime.NumCPU(): %v, %v\n",
				n,
				err,
			)
			n = float64(runtime.NumCPU())
		}
	}()

	// Big enough buffer to read the number in the file wholly into memory.
	buf := make([]byte, 1024)
	var shares float64

	shares, err = readNumberFromFile(sharesPath, buf)
	if err != nil {
		return
	}

	return float64(shares) / denominator
}

func readNumberFromFile(path string, buf []byte) (float64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("runtimebp: failed to open %s: %w", path, err)
	}
	defer file.Close()

	n, err := file.Read(buf)
	if err != nil {
		return 0, fmt.Errorf("runtimebp: failed to read %s: %w", path, err)
	}

	f, err := strconv.ParseInt(strings.TrimSpace(string(buf[:n])), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("runtimebp: failed to parse %s: %w", path, err)
	}
	return float64(f), nil
}

// MaxProcsFormula is the function to calculate GOMAXPROCS based on NumCPU value
// passed in.
type MaxProcsFormula func(n float64) int

func defaultMaxProcsFormula(n float64) int {
	return int(math.Ceil(n*2 - 1))
}

// GOMAXPROCS sets runtime.GOMAXPROCS with the default formula,
// in bound of [min, max].
//
// Currently the default formula is NumCPU()*2-1 rounding up.
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
