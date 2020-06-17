package errorsbp_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/reddit/baseplate.go/errorsbp"
)

func Example() {
	var batch errorsbp.Batch

	var singleError error = batch.Compile()
	fmt.Printf("0: %v\n", singleError)

	err := errors.New("foo")
	batch.Add(err)
	singleError = batch.Compile()
	fmt.Printf("1: %v\n", singleError)

	batch.Add(nil)
	singleError = batch.Compile()
	fmt.Printf("Nil errors are skipped: %v\n", singleError)

	err = errors.New("bar")
	batch.Add(err)
	singleError = batch.Compile()
	fmt.Printf("2: %v\n", singleError)

	var newBatch errorsbp.Batch
	// Add multiple errors at once
	newBatch.Add(
		errors.New("fizz"),
		errors.New("buzz"),
	)
	newBatch.Add(batch)
	fmt.Printf("3: %v\n", newBatch.Compile())

	// Output:
	// 0: <nil>
	// 1: foo
	// Nil errors are skipped: foo
	// 2: errorsbp.Batch: total 2 error(s) in this batch: foo; bar
	// 3: errorsbp.Batch: total 4 error(s) in this batch: fizz; buzz; foo; bar
}

// This example demonstrates how a BatchError can be inspected with errors.Is.
func ExampleBatch_Is() {
	var batch errorsbp.Batch

	batch.Add(context.Canceled)
	err := batch.Compile()
	fmt.Println(errors.Is(err, context.Canceled)) // true
	fmt.Println(errors.Is(err, io.EOF))           // false

	batch.Add(fmt.Errorf("wrapped: %w", io.EOF))
	err = batch.Compile()
	fmt.Println(errors.Is(err, context.Canceled)) // true
	fmt.Println(errors.Is(err, io.EOF))           // true

	// Output:
	// true
	// false
	// true
	// true
}

// This example demonstrates how a BatchError can be inspected with errors.As.
func ExampleBatch_As() {
	var batch errorsbp.Batch
	var target *os.PathError

	batch.Add(context.Canceled)
	err := batch.Compile()
	fmt.Println(errors.As(err, &target)) // false

	batch.Add(fmt.Errorf("wrapped: %w", &os.PathError{}))
	err = batch.Compile()
	fmt.Println(errors.As(err, &target)) // true

	batch.Add(fmt.Errorf("wrapped: %w", &os.LinkError{}))
	err = batch.Compile()
	fmt.Println(errors.As(err, &target)) // true

	// Output:
	// false
	// true
	// true
}
