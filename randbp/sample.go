package randbp

import (
	"math/rand"
)

// ShouldSampleWithRate generates a random float64 in [0, 1) and check it
// against rate.
//
// rate should be in the range of [0, 1].
// When rate <= 0 this function always returns false;
// When rate >= 1 this function always returns true.
func ShouldSampleWithRate(rate float64) bool {
	return rand.Float64() < rate
}
