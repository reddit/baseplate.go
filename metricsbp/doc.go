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
// between pre-created metrics and on-the-fly metrics:
//
//     $ go test -bench . -benchmem
//     goos: darwin
//     goarch: amd64
//     pkg: github.com/reddit/baseplate.go/metricsbp
//     BenchmarkStatsd/pre-create/histogram-8           9938150               123 ns/op              48 B/op          0 allocs/op
//     BenchmarkStatsd/pre-create/counter-8            11073128               115 ns/op              43 B/op          0 allocs/op
//     BenchmarkStatsd/pre-create/gauge-8              11233285               111 ns/op              43 B/op          0 allocs/op
//     BenchmarkStatsd/on-the-fly/histogram-8           4893784               276 ns/op             108 B/op          2 allocs/op
//     BenchmarkStatsd/on-the-fly/counter-8             4845944               270 ns/op             109 B/op          2 allocs/op
//     BenchmarkStatsd/on-the-fly/gauge-8               5656192               209 ns/op              93 B/op          3 allocs/op
//     PASS
//     ok      github.com/reddit/baseplate.go/metricsbp        9.813s
package metricsbp
