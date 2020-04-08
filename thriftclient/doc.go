// Package thriftclient provides a middleware framework for thrift.TClient to
// allow you to automatically run code before and after making a Thrift call.
// It also includes a Middleware to wrap each call in a Thrift client span
// as well as a function that most services can use as the "golden path" for
// setting up a Thrift client pool.
package thriftclient
