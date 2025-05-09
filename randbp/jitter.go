package randbp

import (
	"math/rand/v2"
	"time"
)

// JitterRatio calculates the ratio to be multiplied by base, with +/- jitter.
//
// For example, JitterRatio(0.1) would return a float64 between (0.9, 1.1)
// (exclusive on both ends).
//
// jitter > 1 will be normalized to 1. jitter <= 0 will always return 1.
func JitterRatio(jitter float64) float64 {
	if jitter <= 0 {
		return 1
	}
	if jitter > 1 {
		jitter = 1
	}
	return 1 - (rand.Float64()*2-1)*jitter
}

// JitterDuration applies jitter on the center time duration so the returned
// duration is center +/- jitter.
//
// It uses JitterRatio under-the-hood.  See doc of JitterRatio for more info.
//
// NOTE: If center is very large,
// some precision loss could occur when casting it into float64 to apply jitter,
// but that would only happen when the time duration is prohibitively long.
func JitterDuration(center time.Duration, jitter float64) time.Duration {
	return time.Duration(float64(center) * JitterRatio(jitter))
}
