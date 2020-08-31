package randbp_test

import (
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"

	"github.com/reddit/baseplate.go/randbp"
)

const (
	MinLength = 1
	MaxLength = 20
)

type RandomString string

func (RandomString) Generate(r *rand.Rand, _ int) reflect.Value {
	return reflect.ValueOf(RandomString(randbp.GenerateRandomString(
		randbp.RandomStringArgs{
			R:         r,
			MinLength: MinLength,
			MaxLength: MaxLength,
		},
	)))
}

var _ quick.Generator = RandomString("")

// In real code the function name should be TestRandomString,
// but using that name here will break the example.
func RandomStringTest(t *testing.T) {
	f := func(input RandomString) bool {
		s := string(input)
		if len(s) < MinLength {
			t.Errorf(
				"Expected random string to have a minimal length of %d, got %q",
				MinLength,
				s,
			)
		}
		if len(s) >= MaxLength {
			t.Errorf(
				"Expected random string to have a maximum length of %d, got %q",
				MaxLength,
				s,
			)
		}
		return !t.Failed()
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

// This example demonstrates how to use GenerateRandomString in your tests with
// testing/quick package.
func ExampleGenerateRandomString() {
	// Nothing really here.
	// The real example is on the other functions/types above.
}
