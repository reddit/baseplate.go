package batcherror_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/reddit/baseplate.go/batcherror"
)

func Example() {
	var batch batcherror.BatchError

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

	var newBatch batcherror.BatchError
	err = errors.New("foobar")
	newBatch.Add(err)
	newBatch.Add(batch)
	fmt.Printf("3: %v\n", newBatch.Compile())

	// Output:
	// 0: <nil>
	// 1: foo
	// Nil errors are skipped: foo
	// 2: batcherror: total 2 error(s) in this batch: foo; bar
	// 3: batcherror: total 3 error(s) in this batch: foobar; foo; bar
}

// This example demonstrates how a BatchError can be inspected with errors.Is.
func ExampleBatchError_Is() {
	var batch batcherror.BatchError

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
func ExampleBatchError_As() {
	var batch batcherror.BatchError
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
