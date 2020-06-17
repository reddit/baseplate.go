package errorsbp_test

import (
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"

	"github.com/reddit/baseplate.go/errorsbp"
)

func TestSuppressorNil(t *testing.T) {
	// Test nil safe that it doesn't panic. No real tests here.
	var s errorsbp.Suppressor
	s.Suppress(nil)
	s.Wrap(nil)
}

type specialError struct{}

func (specialError) Error() string {
	return "special error"
}

func specialErrorSuppressor(err error) bool {
	return errors.As(err, new(specialError))
}

type randomError struct {
	err error
}

func (randomError) Generate(r *rand.Rand, _ int) reflect.Value {
	var err error
	if r.Float64() < 0.2 {
		if r.Float64() < 0.5 {
			// For 10% (0.2*0.5) of chance, return specialError
			err = specialError{}
		}
		// For the rest 10%, return nil error
	} else {
		// For the rest 80%, use a random error
		err = fmt.Errorf("random error: %d", r.Int63())
	}
	return reflect.ValueOf(randomError{
		err: err,
	})
}

var (
	_ errorsbp.Suppressor = specialErrorSuppressor
	_ quick.Generator     = randomError{}
)

func TestSuppressor(t *testing.T) {
	t.Run(
		"SuppressNone",
		func(t *testing.T) {
			var s errorsbp.Suppressor

			t.Run(
				"Suppress",
				func(t *testing.T) {
					f := func(e randomError) bool {
						// This one is supposed to return false on all random errors.
						return !s.Suppress(e.err)
					}
					if err := quick.Check(f, nil); err != nil {
						t.Error(err)
					}
				},
			)

			t.Run(
				"Wrap",
				func(t *testing.T) {
					f := func(e randomError) bool {
						wrapped := s.Wrap(e.err)
						if wrapped != e.err {
							t.Errorf("Expected unchanged error %v, got %v", e.err, wrapped)
						}
						return !t.Failed()
					}
					if err := quick.Check(f, nil); err != nil {
						t.Error(err)
					}
				},
			)
		},
	)

	t.Run(
		"specialErrorSuppressor",
		func(t *testing.T) {
			var s errorsbp.Suppressor = specialErrorSuppressor

			t.Run(
				"Suppress",
				func(t *testing.T) {
					f := func(e randomError) bool {
						err := e.err
						expected := errors.As(err, new(specialError))
						actual := s.Suppress(err)
						if actual != expected {
							t.Errorf("Expected %v for err %v, got %v", expected, err, actual)
						}
						return !t.Failed()
					}
					if err := quick.Check(f, nil); err != nil {
						t.Error(err)
					}
				},
			)

			t.Run(
				"Wrap",
				func(t *testing.T) {
					f := func(e randomError) bool {
						err := e.err
						expected := err
						if errors.As(err, new(specialError)) {
							expected = nil
						}
						actual := s.Wrap(err)
						if actual != expected {
							t.Errorf("Expected %v for error %v, got %v", expected, err, actual)
						}
						return !t.Failed()
					}
					if err := quick.Check(f, nil); err != nil {
						t.Error(err)
					}
				},
			)
		},
	)
}

func TestOrSuppressors(t *testing.T) {
	suppressAll := func(err error) bool {
		return true
	}

	t.Run(
		"SuppressNoneOrSpecialErrorSuppressor",
		func(t *testing.T) {
			s := errorsbp.OrSuppressors(specialErrorSuppressor, errorsbp.SuppressNone)

			f := func(e randomError) bool {
				err := e.err
				expected := errors.As(err, new(specialError))
				actual := s.Suppress(err)
				if actual != expected {
					t.Errorf("Expected %v for err %v, got %v", expected, err, actual)
				}
				return !t.Failed()
			}
			if err := quick.Check(f, nil); err != nil {
				t.Error(err)
			}
		},
	)

	t.Run(
		"SpecialErrorSuppressorOrSuppressAll",
		func(t *testing.T) {
			s := errorsbp.OrSuppressors(specialErrorSuppressor, suppressAll)

			f := func(e randomError) bool {
				// This one is supposed to return true on all random errors.
				return s.Suppress(e.err)
			}
			if err := quick.Check(f, nil); err != nil {
				t.Error(err)
			}
		},
	)
}
