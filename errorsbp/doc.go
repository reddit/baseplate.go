// Package errorsbp provides BatchError, which can be used to compile multiple
// errors into a single one.
//
// An example of how to use it in your functions:
//
//     type worker func() error
//
//     func runWorksParallel(works []worker) error {
//         errChan := make(chan error, len(works))
//         var wg sync.WaitGroup
//         wg.Add(len(works))
//
//         for _, work := range works {
//             go func(work worker) {
//                 defer wg.Done()
//                 errChan <- work()
//             }(work)
//         }
//
//         wg.Wait()
//         var batch errorsbp.BatchError
//         for err := range errChan {
//             // nil errors will be auto skipped
//             batch.Add(err)
//         }
//         // If all works succeeded, Compile() returns nil.
//         // If only one work failed, Compile() returns that error directly
//         // instead of wrapping it inside BatchError.
//         return batch.Compile()
//     }
//
// This package is not thread-safe.
// The same batch should not be operated on different goroutines concurrently.
package errorsbp
