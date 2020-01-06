// Package metricsbp provides metrics related features for baseplate.go,
// based on go-kit metrics package.
//
// There are two parts of this package:
// 1. Wrappers of go-kit metrics to provide easy to use create on-the-fly
// metrics, similar to what we have in baseplate.py, but they are usually
// between 2x and 3x slower compare to using pre-created metrics.
// 2. Helper function for use cases of pre-create the metrics before using them.
//
// This package comes with benchmark test to show the performance difference
// between pre-created metrics, on-the-fly metrics, and on-the-fly with
// additional labels metrics:
//
//     $ go test -bench . -benchmem
//     goos: darwin
//     goarch: amd64
//     pkg: github.com/reddit/baseplate.go/metricsbp
//     BenchmarkStatsd/pre-create/histogram-8           9548755               121 ns/op              39 B/op          0 allocs/op
//     BenchmarkStatsd/pre-create/counter-8            10924286               122 ns/op              44 B/op          0 allocs/op
//     BenchmarkStatsd/pre-create/gauge-8              100000000               10.7 ns/op             0 B/op          0 allocs/op
//     BenchmarkStatsd/on-the-fly/histogram-8           4342429               247 ns/op              94 B/op          2 allocs/op
//     BenchmarkStatsd/on-the-fly/counter-8             4850746               263 ns/op             125 B/op          2 allocs/op
//     BenchmarkStatsd/on-the-fly/gauge-8              29172970                40.7 ns/op             0 B/op          0 allocs/op
//     BenchmarkStatsd/on-the-fly-with-labels/histogram-8               2687974               449 ns/op             191 B/op          4 allocs/op
//     BenchmarkStatsd/on-the-fly-with-labels/counter-8                 2680872               448 ns/op             191 B/op          4 allocs/op
//     BenchmarkStatsd/on-the-fly-with-labels/gauge-8                   3645584               331 ns/op             112 B/op          3 allocs/op
//     PASS
//     ok      github.com/reddit/baseplate.go/metricsbp        14.045s
package metricsbp
