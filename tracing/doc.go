// Package tracing provides tracing integration with zipkin.
//
// For thrift services, they should call StartSpanFromThriftContext with the
// context object injected by thrift library to get a root span for the thrift
// handler function, then call CreateChildSpan to create child-spans.
//
// This package also implements opentracing defined interfaces.
// As a side effect,
// importing this package will call opentracing.SetGlobalTracer automatically
// with a Tracer implementation that does not send spans anywhere.
// Call InitGlobalTracer early in your main function to setup spans sampling.
package tracing
