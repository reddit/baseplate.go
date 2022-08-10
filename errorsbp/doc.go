// Package errorsbp provides some error utilities for Baseplate.go project.
//
// # Batch
//
// Batch can be used to compile multiple errors into a single one.
//
// An example of how to use it in your functions:
//
//	type worker func() error
//
//	func runWorksParallel(works []worker) error {
//	    errChan := make(chan error, len(works))
//	    var wg sync.WaitGroup
//	    wg.Add(len(works))
//
//	    for _, work := range works {
//	        go func(work worker) {
//	            defer wg.Done()
//	            errChan <- work()
//	        }(work)
//	    }
//
//	    wg.Wait()
//	    close(errChan)
//	    var batch errorsbp.Batch
//	    for err := range errChan {
//	        // nil errors will be auto skipped
//	        batch.Add(err)
//	    }
//	    // If all works succeeded, Compile() returns nil.
//	    // If only one work failed, Compile() returns that error directly
//	    // instead of wrapping it inside BatchError.
//	    return batch.Compile()
//	}
//
// Batch is not thread-safe.
// The same batch should not be operated on different goroutines concurrently.
//
// # Suppressor
//
// Suppressor is a type defined to provide an unified way to allow certain
// functions/features to ignore certain errors.
//
// It's currently used by thriftbp package in both server and client
// middlewares, to not treat certain errors defined in thrift IDL as span
// errors. Because of go's type system, we cannot reliably provide a Suppressor
// implementation to suppress all errors defined in all thrift IDLs, as a result
// we rely on service/client implementations to implement it for the
// middlewares.
//
// An example of how to implement it for your thrift defined errors:
//
//	func MyThriftSuppressor(err error) bool {
//	    return errors.As(err, new(*mythrift.MyThriftErrorType))
//	}
package errorsbp
