// Package internalv2compat is an INTERNAL ONLY library provided for transitional projects that need
// to use baseplate v2 and v0 in the same module.
//
// DO NOT USE THIS LIBRARY DIRECTLY.  Breaking changes may be made to this package at any time.
//
// Deprecated: This is an internal library and should not be used directly.
package internalv2compat

import (
	"net/http"
	"os"
	"sync"
	"sync/atomic"

	"github.com/apache/thrift/lib/go/thrift"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// globalLogger() is used by all top-level log methods (e.g. Infof) in the log package.
//
// Before the Init methods are called, this logger will use a very basic config
// that includes a `pre_init` key to distinguish this condition.
var globalLogger = zap.New(
	zapcore.NewCore(
		zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
		os.Stderr,
		zapcore.DebugLevel,
	),
	zap.Fields(
		zap.Bool("pre_init", true),
	),
).WithOptions(
	zap.AddCallerSkip(1), // will always be called via a top-level function from this package
).Sugar()

// GlobalLogger is an inlineable accessor for the global logger to allow for injection from baseplate v2.
func GlobalLogger() *zap.SugaredLogger {
	return globalLogger
}

var v0logDisabled atomic.Uint64

// SetGlobalLogger allows baseplate v0 to set the global logger.
func SetGlobalLogger(logger *zap.SugaredLogger) {
	if v0logDisabled.Load() > 0 {
		globalLogger.Warn("ineffectual call to SetGlobalLogger; baseplate.go v2 compatibility mode is active")
		return
	}

	globalLogger = logger
}

// OverrideLogger allows baseplate v2 to override the global logger irrevocably.
func OverrideLogger(logger *zap.SugaredLogger) {
	v0logDisabled.Store(1)
	globalLogger = logger
}

// IsHTTP allows detecting the unexported httpbp.server without resorting to reflection.
type IsHTTP interface {
	isHTTP()
}

// IsThrift allows detecting the unexported thriftbp.server without resorting to reflection.
type IsThrift interface {
	isThrift()
}

// ClientTraceMiddlewareArgs is the arguments used to instantiate client tracing middleware
//
// This struct is exported so that it can be used by baseplate V2 interop utilities.
type ClientTraceMiddlewareArgs struct {
	ServiceName string
}

type ThriftClientTraceMiddlewareProvider func(args ClientTraceMiddlewareArgs) thrift.ClientMiddleware

var v2Tracing struct {
	sync.Mutex
	enabled bool

	thriftClientMiddlewareProvider ThriftClientTraceMiddlewareProvider
	thriftServerMiddleware         thrift.ProcessorMiddleware

	httpClientMiddleware func(base http.RoundTripper) http.RoundTripper
	httpServerMiddleware func(name string, next http.Handler) http.Handler
}

func SetV2TracingEnabled(enabled bool) {
	v2Tracing.Lock()
	defer v2Tracing.Unlock()
	v2Tracing.enabled = enabled
}

func V2TracingEnabled() bool {
	v2Tracing.Lock()
	defer v2Tracing.Unlock()
	return v2Tracing.enabled
}

func SetV2TracingThriftClientMiddleware(middleware thrift.ClientMiddleware) {
	v2Tracing.Lock()
	defer v2Tracing.Unlock()
	v2Tracing.thriftClientMiddlewareProvider = func(args ClientTraceMiddlewareArgs) thrift.ClientMiddleware {
		return middleware
	}
}

func SetV2TracingThriftClientMiddlewareProvider(provider ThriftClientTraceMiddlewareProvider) {
	v2Tracing.Lock()
	defer v2Tracing.Unlock()
	v2Tracing.thriftClientMiddlewareProvider = provider
}

func V2TracingThriftClientMiddleware() thrift.ClientMiddleware {
	v2Tracing.Lock()
	defer v2Tracing.Unlock()
	if v2Tracing.thriftClientMiddlewareProvider == nil {
		return nil
	}
	return v2Tracing.thriftClientMiddlewareProvider(ClientTraceMiddlewareArgs{ServiceName: "unknown"})
}

func V2TracingThriftClientMiddlewareWithArgs(args ClientTraceMiddlewareArgs) thrift.ClientMiddleware {
	v2Tracing.Lock()
	defer v2Tracing.Unlock()
	if v2Tracing.thriftClientMiddlewareProvider == nil {
		return nil
	}
	return v2Tracing.thriftClientMiddlewareProvider(args)
}

func SetV2TracingThriftServerMiddleware(middleware thrift.ProcessorMiddleware) {
	v2Tracing.Lock()
	defer v2Tracing.Unlock()
	v2Tracing.thriftServerMiddleware = middleware
}

func V2TracingThriftServerMiddleware() thrift.ProcessorMiddleware {
	v2Tracing.Lock()
	defer v2Tracing.Unlock()
	return v2Tracing.thriftServerMiddleware
}

func SetV2TracingHTTPClientMiddleware(middleware func(base http.RoundTripper) http.RoundTripper) {
	v2Tracing.Lock()
	defer v2Tracing.Unlock()
	v2Tracing.httpClientMiddleware = middleware
}

func V2TracingHTTPClientMiddleware() func(base http.RoundTripper) http.RoundTripper {
	v2Tracing.Lock()
	defer v2Tracing.Unlock()
	return v2Tracing.httpClientMiddleware
}

func SetV2TracingHTTPServerMiddleware(middleware func(name string, next http.Handler) http.Handler) {
	v2Tracing.Lock()
	defer v2Tracing.Unlock()
	v2Tracing.httpServerMiddleware = middleware
}

func V2TracingHTTPServerMiddleware() func(name string, next http.Handler) http.Handler {
	v2Tracing.Lock()
	defer v2Tracing.Unlock()
	return v2Tracing.httpServerMiddleware
}
