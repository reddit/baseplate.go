// Package metricsbp provides metrics related features for baseplate.go,
// based on go-kit metrics package.
//
// There are three main parts of this package:
//
// 1. Wrappers of go-kit metrics to provide easy to use create on-the-fly
// metrics, similar to what we have in Baseplate.py.
//
// 2. Helper function for use cases of pre-create the metrics before using them.
//
// 3. Sampled counter/histogram implementations.
//
// This package comes with benchmark test to show the performance difference
// between pre-created metrics, on-the-fly metrics, and on-the-fly with
// additional labels metrics:
//
//     $ go test -bench . -benchmem
//     goos: darwin
//     goarch: amd64
//     pkg: github.com/reddit/baseplate.go/metricsbp
//     BenchmarkStatsd/pre-create/histogram-8         	 8583646	       124 ns/op	      44 B/op	       0 allocs/op
//     BenchmarkStatsd/pre-create/timing-8            	10221859	       120 ns/op	      47 B/op	       0 allocs/op
//     BenchmarkStatsd/pre-create/counter-8           	10205341	       120 ns/op	      47 B/op	       0 allocs/op
//     BenchmarkStatsd/pre-create/gauge-8             	96462238	        12.4 ns/op	       0 B/op	       0 allocs/op
//     BenchmarkStatsd/on-the-fly/histogram-8         	 4665778	       256 ns/op	      99 B/op	       2 allocs/op
//     BenchmarkStatsd/on-the-fly/timing-8            	 4784816	       273 ns/op	     126 B/op	       2 allocs/op
//     BenchmarkStatsd/on-the-fly/counter-8           	 4818908	       259 ns/op	     125 B/op	       2 allocs/op
//     BenchmarkStatsd/on-the-fly/gauge-8             	28754060	        40.6 ns/op	       0 B/op	       0 allocs/op
//     BenchmarkStatsd/on-the-fly-with-labels/histogram-8         	 2624264	       453 ns/op	     192 B/op	       4 allocs/op
//     BenchmarkStatsd/on-the-fly-with-labels/timing-8            	 2639377	       449 ns/op	     192 B/op	       4 allocs/op
//     BenchmarkStatsd/on-the-fly-with-labels/counter-8           	 2600418	       457 ns/op	     193 B/op	       4 allocs/op
//     BenchmarkStatsd/on-the-fly-with-labels/gauge-8             	 3429901	       339 ns/op	     112 B/op	       3 allocs/op
//     PASS
//     ok  	github.com/reddit/baseplate.go/metricsbp	18.675s
package metricsbp
