package randbp_test

import (
	"testing"
	"testing/quick"

	"github.com/reddit/baseplate.go/randbp"
)

func TestShouldSampleWithRate(t *testing.T) {
	t.Run(
		"0",
		func(t *testing.T) {
			if randbp.ShouldSampleWithRate(0) {
				t.Error("randbp.ShouldSampleWithRate(0) returned true")
			}

			f := func() bool {
				return !randbp.ShouldSampleWithRate(0)
			}
			if err := quick.Check(f, nil); err != nil {
				t.Error(err)
			}
		},
	)

	t.Run(
		"1",
		func(t *testing.T) {
			if !randbp.ShouldSampleWithRate(1) {
				t.Error("randbp.ShouldSampleWithRate(1) returned false")
			}

			f := func() bool {
				return randbp.ShouldSampleWithRate(1)
			}
			if err := quick.Check(f, nil); err != nil {
				t.Error(err)
			}
		},
	)
}

func BenchmarkShouldSampleWithRate(b *testing.B) {
	for i := 0; i < b.N; i++ {
		randbp.ShouldSampleWithRate(0)
	}
}
