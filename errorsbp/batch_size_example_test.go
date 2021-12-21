package errorsbp_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/reddit/baseplate.go/errorsbp"
)

var (
	errOne   = errors.New("dummy error #1")
	errTwo   = errors.New("dummy error #2")
	errThree = errors.New("dummy error #3")
)

// MyFunction is the function to be tested by MyFunctionTest.
func MyFunction(i int) error {
	var be errorsbp.Batch
	switch i {
	case 0:
		// do nothing
	case 1:
		be.Add(errOne)
	case 2:
		be.Add(errOne)
		be.Add(errTwo)
	case 3:
		be.Add(errOne)
		be.Add(errTwo)
		be.Add(errThree)
	}
	return be.Compile()
}

// NOTE: In real unit test code this function signature should be:
//
//     func TestMyFunction(t *testing.T)
//
// But doing that will break this example.
func MyFunctionTest() {
	var (
		t *testing.T
	)
	for _, c := range []struct {
		arg  int
		want []error
	}{
		{
			arg: 0,
		},
		{
			arg:  1,
			want: []error{errOne},
		},
		{
			arg:  2,
			want: []error{errOne, errTwo},
		},
		{
			arg:  3,
			want: []error{errOne, errTwo, errThree},
		},
	} {
		t.Run(fmt.Sprintf("%v", c.arg), func(t *testing.T) {
			got := MyFunction(c.arg)
			t.Logf("got error: %v", got)
			if len(c.want) != errorsbp.BatchSize(got) {
				t.Errorf("Expected %d errors, got %d", len(c.want), errorsbp.BatchSize(got))
			}
			for _, target := range c.want {
				if !errors.Is(got, target) {
					t.Errorf("Expected error %v to be returned but it's not", target)
				}
			}
		})
	}
}

// This example demonstrates how to use errorsbp.BatchSize in a unit test.
func ExampleBatchSize() {
	// See MyFuncionTest above for the real example.
}
