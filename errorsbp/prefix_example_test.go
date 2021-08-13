package errorsbp_test

import (
	"errors"
	"fmt"

	"github.com/reddit/baseplate.go/errorsbp"
)

// This example demonstrates how to use PrefixError.
func ExamplePrefixError() {
	// Two worker functions that could potentially return error.
	const works = 2
	var (
		workerA = func() error {
			// workerA will succeed, and it will be auto skipped in the batch.
			return nil
		}
		workerB = func() error {
			// workerB will fail, and the error will be prefixed in the batch.
			return errors.New("b")
		}
	)
	errs := make(chan error, works)
	go func() {
		errs <- errorsbp.PrefixError("workerA", workerA())
	}()
	go func() {
		errs <- errorsbp.PrefixError("workerB", workerB())
	}()

	var batch errorsbp.Batch
	for i := 0; i < works; i++ {
		batch.Add(<-errs)
	}
	fmt.Println(batch.Compile())

	// Output:
	// workerB: b
}
