// Package clientpool provides implementations of a generic client pool.
//
// The channel implementation has roughly 300ns overhead on Get/Release pair
// calls:
//
//     $ go test -bench . -benchmem
//     goos: darwin
//     goarch: amd64
//     pkg: github.com/reddit/baseplate.go/clientpool
//     BenchmarkPoolGetRelease/channel-8         	 3993308	       278 ns/op	       0 B/op	       0 allocs/op
//     PASS
//     ok  	github.com/reddit/baseplate.go/clientpool	2.495s
//
// This package is considered low level.
// Package thriftclient provided a more useful,
// thrift-specific wrapping to this package.
// In most cases you would want to use that package instead if you are using the
// pool for thrift clients.
// See thriftclient package's doc for examples.
package clientpool
