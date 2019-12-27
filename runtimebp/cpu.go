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
		if err != nil {
			// Fallback and log to stderr.
			n = float64(runtime.NumCPU())
			fmt.Fprintf(
				os.Stderr,
				"NumCPU: falling back to use runtime.NumCPU(): %v\n",
				err,
			)
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

func readNumberFromFile(path string, buf []byte) (float64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("failed to open %s: %w", path, err)
	}
	defer file.Close()

	n, err := file.Read(buf)
	if err != nil {
		return 0, fmt.Errorf("failed to read %s: %w", path, err)
	}

	f, err := strconv.ParseInt(strings.TrimSpace(string(buf[:n])), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	return float64(f), nil
}

// GOMAXPROCS sets runtime.GOMAXPROCS.
//
// It uses NumCPU()*2-1 rounding up, in bound of [min, max].
func GOMAXPROCS(min, max int) int {
	n := math.Ceil(NumCPU()*2 - 1)
	return runtime.GOMAXPROCS(boundNtoMinMax(int(n), min, max))
}

func boundNtoMinMax(n, min, max int) int {
	ret := float64(n)
	ret = math.Max(float64(min), ret)
	ret = math.Min(float64(max), ret)
	return int(ret)
}
