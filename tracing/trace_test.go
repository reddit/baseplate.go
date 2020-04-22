package tracing

import (
	"testing"
	"testing/quick"
)

func TestNonZeroRandUint64(t *testing.T) {
	f := func() bool {
		id := nonZeroRandUint64()
		return id != 0
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}
