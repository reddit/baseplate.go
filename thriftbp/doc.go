// Package thriftbp provides Baseplate specific thrift related helpers.
//
// Clients
//
// On the client side,
// this package provides a middleware framework for thrift.TClient to allow you
// to automatically run code before and after making a Thrift call.
// It also includes middleware implementations to wrap each call in a Thrift
// client span as well as a function that most services can use as the
// "golden path" for setting up a Thrift client pool.
//
// Servers
//
// On the server side,
// this package provides middleware implementations for EdgeRequestContext
// handling and tracing propagation according to Baseplate spec.
package thriftbp
