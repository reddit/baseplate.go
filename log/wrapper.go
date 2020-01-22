package log

import (
	stdlog "log"
	"testing"
)

// Wrapper is a simple wrapper of a logging function.
//
// In reality we might actually use different logging libraries in different
// services, and they are not always compatible with each other.
// Wrapper is a simple common ground that it's easy to wrap whatever logging
// library we use into.
//
// This is also the same type as thrift.Logger and can be used interchangeably
// (sometimes with a typecast).
type Wrapper func(msg string)

// NopWrapper is a Wrapper implementation that does nothing.
func NopWrapper(msg string) {}

// StdWrapper wraps stdlib log package into a Wrapper.
func StdWrapper(logger *stdlog.Logger) Wrapper {
	if logger == nil {
		return NopWrapper
	}
	return func(msg string) {
		logger.Print(msg)
	}
}

// TestWrapper is a wrapper can be used in test codes.
//
// It fails the test when called.
func TestWrapper(tb testing.TB) Wrapper {
	return func(msg string) {
		tb.Errorf("logger called with msg: %q", msg)
	}
}

// ZapWrapper wraps zap log package into a Wrapper.
func ZapWrapper(level Level) Wrapper {
	// For unknown values, fallback to info level.
	f := Info
	switch level {
	case DebugLevel:
		f = Debug
	case WarnLevel:
		f = Warn
	case ErrorLevel:
		f = Error
	case PanicLevel:
		f = Panic
	case FatalLevel:
		f = Fatal
	case NopLevel:
		return NopWrapper
	}
	return func(msg string) {
		f(msg)
	}
}
