// Package headerbp provides the shared code for propagating baseplate headers using server and client middlewares.
//
// It is meant to be used by middlewares for different rpc frameworks like http and grpc, not used directly by services.
//
// It is only meant to propagate headers that the server receives, the client middlewares will return an error if they
// detect a baseplate header in the request being sent.
package headerbp
