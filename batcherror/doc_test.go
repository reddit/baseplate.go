package batcherror_test

import (
	"errors"
	"fmt"

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
