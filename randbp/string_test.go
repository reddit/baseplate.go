package randbp_test

import (
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"

	"github.com/reddit/baseplate.go/randbp"
)

func TestGenerateRandomStringNil(t *testing.T) {
	// Just make sure it doesn't panic when all optional args are absent.
	// No real tests here.
	randbp.GenerateRandomString(randbp.RandomStringArgs{
		MaxLength: 10,
	})
}

const (
	minLength = 1
	maxLength = 20
)

type randomString string

func (randomString) Generate(r *rand.Rand, _ int) reflect.Value {
	return reflect.ValueOf(randomString(randbp.GenerateRandomString(
		randbp.RandomStringArgs{
			R:         r,
			MinLength: minLength,
			MaxLength: maxLength,
		},
	)))
}

var _ quick.Generator = randomString("")

func TestRandomStringQuick(t *testing.T) {
	f := func(input randomString) bool {
		s := string(input)
		if len(s) < minLength {
			t.Errorf(
				"Expected random string to have a minimal length of %d, got %q",
				minLength,
				s,
			)
		}
		if len(s) >= maxLength {
			t.Errorf(
				"Expected random string to have a maximum length of %d, got %q",
				maxLength,
				s,
			)
		}
		return !t.Failed()
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}
