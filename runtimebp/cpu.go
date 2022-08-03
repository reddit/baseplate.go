package runtimebp

import (
	"fmt"
	"io"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// NumCPU returns the number of CPUs assigned to this running container.
//
// This is the container aware version of runtime.NumCPU.
// It reads from the cgroup v2 cpu.max values to determine the hard CPU limit of
// the container.
//
// If the current process is not running with cgroup v2,
// it falls back to read from the cgroup v1 cpu.cfs_quota_us and
// cpu.cfs_period_us values.
// If the current process is not running inside a container,
// or if there's no limit set in cgroup,
// it will fallback to runtime.NumCPU() instead.
//
// Depending on your application, $BASEPLATE_CPU_REQUEST may also be helpful.
// Infrared sets $BASEPLATE_CPU_REQUEST to the container's Kubernetes CPU
// _request_ (rather than _limit_ as this function returns), rounded up to the
// nearest whole CPU.
//
// To tune GOMAXPROCS, see runtimebp.InitFromConfig.
func NumCPU() float64 {
	// Big enough buffer to read the numbers in the files wholly into memory.
	buf := make([]byte, 1024)

	n, err := numCPUCgroupsV2(buf)
	if err == nil {
		return n
	}

	// fallback to cgroups v1
	fmt.Fprintf(
		os.Stderr,
		"runtimebp.NumCPU: Failed to read cgroup v2, fallback to cgroup v1: %v\n",
		err,
	)
	n, err = numCPUCgroupsV1(buf)
	if err == nil {
		return n
	}

	// Fallback to the standard runtime package value.
	fmt.Fprintf(
		os.Stderr,
		"runtimebp.NumCPU: Failed to read cgroup v1, fallback to NumCPU on the physical machine: %v\n",
		err,
	)
	return float64(runtime.NumCPU())
}

func numCPUCgroupsV2(buf []byte) (float64, error) {
	const (
		maxPath = "/sys/fs/cgroup/cpu.max"
	)

	values, err := readNumbersFromFile(maxPath, buf, 2)
	if err != nil {
		return 0, fmt.Errorf("failed to read max file: %w", err)
	}

	return values[0] / values[1], nil
}

func numCPUCgroupsV1(buf []byte) (float64, error) {
	const (
		quotaPath  = "/sys/fs/cgroup/cpu/cpu.cfs_quota_us"
		periodPath = "/sys/fs/cgroup/cpu/cpu.cfs_period_us"
	)

	quota, err := readNumbersFromFile(quotaPath, buf, 1)
	if err != nil || len(quota) != 1 {
		return 0, fmt.Errorf("failed to read quota file: %w", err)
	}

	// CFS quota returns -1 if there is no limit, return the default.
	if quota[0] < 0 {
		return 0, fmt.Errorf(
			"quota file returned %f",
			quota[0],
		)
	}

	period, err := readNumbersFromFile(periodPath, buf, 1)
	if err != nil {
		return 0, fmt.Errorf("failed to read period file: %w", err)
	}

	return quota[0] / period[0], nil
}

func readNumbersFromFile(path string, buf []byte, numbers int) ([]float64, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open %q: %w", path, err)
	}
	defer file.Close()

	n, err := io.ReadFull(file, buf)
	if err != nil && err != io.ErrUnexpectedEOF {
		return nil, fmt.Errorf("failed to read %q: %w", path, err)
	}

	line := strings.TrimSpace(string(buf[:n]))
	strs := strings.Fields(line)
	if numbers != len(strs) {
		return nil, fmt.Errorf("got %d numbers instead of %d: %q", len(strs), numbers, line)
	}
	result := make([]float64, numbers)
	for i, s := range strs {
		f, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %q: %w (#%d of %q)", path, err, i, line)
		}
		result[i] = float64(f)
	}
	return result, nil
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
//
// Deprecated: GOMAXPROCS is deprecated. Instead, tune GOMAXPROCS as described in runtimebp.InitFromConfig.
func GOMAXPROCS(min, max int) (oldVal, newVal int) {
	return GOMAXPROCSwithFormula(min, max, defaultMaxProcsFormula)
}

// GOMAXPROCSwithFormula sets runtime.GOMAXPROCS with the given formula,
// in bound of [min, max].
//
// Deprecated: GOMAXPROCSwithFormula is deprecated. Instead, tune GOMAXPROCS as described in runtimebp.InitFromConfig.
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
