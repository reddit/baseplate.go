// Package thriftpool provides implementations of thrift client pool.
//
// The channel implementation has roughly 300ns overhead on Get/Release pair
// calls:
//
//     $ go test -bench . -benchmem
//     goos: darwin
//     goarch: amd64
//     pkg: github.com/reddit/baseplate.go/thriftpool
//     BenchmarkPoolGetRelease/channel-8         	 3993308	       278 ns/op	       0 B/op	       0 allocs/op
//     PASS
//     ok  	github.com/reddit/baseplate.go/thriftpool	2.495s
package thriftpool
