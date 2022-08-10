// Package clientpool provides implementations of a generic client pool.
//
// The channel implementation has roughly 300ns overhead on Get/Release pair
// calls:
//
//	$ go test -bench . -benchmem
//	goos: darwin
//	goarch: amd64
//	pkg: github.com/reddit/baseplate.go/clientpool
//	BenchmarkPoolGetRelease/channel-8         	 3993308	       278 ns/op	       0 B/op	       0 allocs/op
//	PASS
//	ok  	github.com/reddit/baseplate.go/clientpool	2.495s
//
// This package is considered low level and should not be used directly in most
// cases.
// A thrift-specific wrapping is available in thriftbp package.
package clientpool
