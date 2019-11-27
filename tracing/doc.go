// Package tracing provides tracing integration with zipkin.
//
// For thrift services, they should call StartSpanFromThriftContext with the
// context object injected by thrift library to get a root span for the thrift
// handler function, then call StartSpanFromParent to create subspans.
package tracing
