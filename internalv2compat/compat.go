// Package internalv2compat is an INTERNAL ONLY library provided for transitional projects that need
// to use baseplate v2 and v0 in the same module.
//
// DO NOT USE THIS LIBRARY DIRECTLY.  Breaking changes may be made to this package at any time.
//
// Deprecated: This is an internal library and should not be used directly.
package internalv2compat

import (
	"os"
	"sync"
	"sync/atomic"

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

var (
	v2TracingEnabledMutex sync.RWMutex
	v2TracingEnabled      bool
)

func SetV2TracingEnabled(enabled bool) {
	v2TracingEnabledMutex.Lock()
	defer v2TracingEnabledMutex.Unlock()
	v2TracingEnabled = enabled
}
func V2TracingEnabled() bool {
	v2TracingEnabledMutex.RLock()
	defer v2TracingEnabledMutex.RUnlock()
	return v2TracingEnabled
}
