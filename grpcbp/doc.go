// Package grpcbp provides Baseplate specific gRPC related helpers.
//
// # Clients
//
// On the client side, this package provides middlewares to support tracing
// propagation or initialization as well as forwarding EdgeRequestContext
// according to the Baseplate specification.
//
// # Servers
//
// On the server side, this package provides middleware implementations for
// EdgeRequestContext handling and tracing propagation according to Baseplate
// specification.
package grpcbp
