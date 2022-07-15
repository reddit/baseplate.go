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

const defaultMultiplier = 2

var millicoreRegexp = regexp.MustCompile(`^([1-9][0-9]*)m$`)

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
		"runtimebp.NumCPU: Failed to read cgroup v1, fallback to %d to allow parallelism: %v\n",
		defaultMultiplier,
		err,
	)
	return defaultMultiplier
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
	// If we have a request below 1, round up so we don't end up single-threaded
	if n < 1 {
		n = 1
	}
	i := fetchMaxProcsMult()
	return int(math.Ceil(n * float64(i)))
}

func fetchMaxProcsMult() int {
	// Allow using a multiplier for number of processes relative to limit.
	// Not catching the ok here because this function will never be called unless
	// the environment variable is set
	v, _ := os.LookupEnv("BASEPLATE_GOMAXPROCSMULT")
	intVal, err := strconv.Atoi(v)
	if err != nil {
		fmt.Fprintf(
			os.Stderr,
			"runtimebp.fetchMaxProcsMult: GOMAXPROCSMULT is set to %q, which is invalid, falling back to %d to ensure parallelism: %v",
			v,
			defaultMultiplier,
			err,
		)
		intVal = defaultMultiplier
	}
	if intVal <= 0 {
		intVal = 1
	}
	return intVal
}

// fetchCPURequest checks for the magic BASEPLATE_CPU_REQUEST variable set as a fallback
// by Infrared.  If this is set, we use it and the multiplier to set our
// GOMAXPROCS as a fallback.
func fetchCPURequest() (int, bool) {
	var req float64
	v, ok := os.LookupEnv("BASEPLATE_CPU_REQUEST")
	if !ok {
		return 0, false
	}

	iReq, err := strconv.Atoi(v)
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
				"runtimebp.fetchCPURequest: BASEPLATE_CPU_REQUEST is set to %q, which is invalid, ignoring.",
				v,
			)
			req = 1
		} else {
			m, err := strconv.Atoi(match[1])
			if err == nil {
				req = float64(m) / 1000
			} else {
				fmt.Fprintf(
					os.Stderr,
					"runtimebp.fetchCPURequest: BASEPLATE_CPU_REQUEST is set and appears to be a millicore value (%v), but not an integer: %v",
					req,
					err,
				)
				req = 1
			}
		}
	} else {
		req = float64(iReq)
	}
	return scaledMaxProcsFormula(req), true
}

// GOMAXPROCS sets runtime.GOMAXPROCS with the relevant formula,
// in bound of [min, max].
//
// Start by checking if limits are set at all.
// If they aren't, fall back to the CPU request for the container
// as provided by Infrared, with multiplier, if it exists.
// If limits are set, check for a multiplier and multiply by it.
// If limits are set with no multiplier, use a default multiplier.
// If no limits are set and no request is provided by Infrared,
// just use the default multiplier outright as our GOMAXPROCS value.
func GOMAXPROCS(min, max int) (oldVal, newVal int) {
	var limitSet error
	buf := make([]byte, 1024)
	_, limitSet = numCPUCgroupsV2(buf)
	if limitSet == nil {
		_, limitSet = numCPUCgroupsV1(buf)
		if limitSet == nil {
			if req, ok := fetchCPURequest(); ok {
				oldVal = runtime.GOMAXPROCS(req)
				return oldVal, req
			}
		}
	}
	if _, ok := os.LookupEnv("BASEPLATE_GOMAXPROCSMULT"); ok {
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
