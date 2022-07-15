package runtimebp

import (
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

var millicoreRegexp = regexp.MustCompile(`^(P<millis>[1-9][0-9]*)m$`)

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

	// If nothing is set, falling back to the core count is probably dangerous.
	// Instead, fall back to the lowest value that still allows parallelism.
	fmt.Fprintf(
		os.Stderr,
		"runtimebp.NumCPU: Failed to read cgroup v1, fallback to 2 to allow parallelism: %v\n",
		err,
	)
	return 2
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

func scaledMaxProcsFormula(n float64) int {
	i := fetchMaxProcsMult()
	return int(math.Ceil(n) * float64(i))
}

func fetchMaxProcsMult() int {
	// Allow using a multiplier for number of processes relative to limit.
	// Not catching the ok here because this function will never be called unless
	// the environment variable is set
	v, _ := os.LookupEnv("GOMAXPROCSMULT")
	i, err := strconv.Atoi(v)
	if err != nil {
		fmt.Fprintf(
			os.Stderr,
			"runtimebp.fetchMaxProcsMult: GOMAXPROCSMULT is set to %q, which is invalid, falling back to 2 to ensure parallelism: %v",
			v,
			err,
		)
		i = 2
	}
	if i <= 0 {
		i = 1
	}
	return i
}

// fetchCpuRequest checks for the magic CPUREQUEST variable set as a fallback
// by Infrared.  If this is set, we use it and the multiplier to set our
// GOMAXPROCS as a fallback.
func fetchCPURequest() (int, error) {
	if v, ok := os.LookupEnv("CPUREQUEST"); ok {
		req, err := strconv.Atoi(v)
		if err != nil {
			// This case will probably never be hit, but I'm keeping it just in case.
			// In k8s 1.21 and newer, when you present the CPU request via the Downward API
			// k8s automatically rounds it up to the nearest integer unit.  That's how
			// we plan to set this variable, but just in case it might be worth doing this
			// check in case they ever break that.  There's no official contract for that.
			match := millicoreRegexp.FindStringSubmatch(v)
			if match == nil {
				fmt.Fprintf(
					os.Stderr,
					"runtimebp.fetchCPURequest: CPUREQUEST is set to %q, which is invalid, ignoring.",
					v,
				)
				req = 1
			} else {
				m, err := strconv.Atoi(match[0])
				if err != nil {
					req = int(math.Ceil(float64(m) / 1000.0))
				} else {
					fmt.Fprintf(
						os.Stderr,
						"runtimebp.fetchCPURequest: CPUREQUEST is set and appears to be a millicore value (%v), but not an integer: %v",
						req,
						err,
					)
					req = 1
				}
			}
		}
		return scaledMaxProcsFormula(float64(req)), nil
	}
	return 0, fmt.Errorf("CPUREQUEST unset")
}

// GOMAXPROCS sets runtime.GOMAXPROCS with the formula,
// in bound of [min, max].
//
// Currently the default formula is NumCPU() rounding up.
func GOMAXPROCS(min, max int) (oldVal, newVal int) {
	if req, err := fetchCPURequest(); err == nil {
		return GOMAXPROCSwithFormula(req, req, defaultMaxProcsFormula)
	}
	if _, ok := os.LookupEnv("GOMAXPROCSMULT"); ok {
		return GOMAXPROCSwithFormula(min, max, scaledMaxProcsFormula)
	}
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
