package log

import (
	"errors"
	stdlog "log"
	"testing"

	sentry "github.com/getsentry/sentry-go"
)

// Wrapper defines a simple interface to wrap logging functions.
//
// As principles, library code should
//
// 1. Not do any logging.
// The library code should communicate errors back to the caller,
// and let the caller decide how to deal with them
// (log them, ignore them, panic, etc.)
//
// 2. In some rare cases, 1 is not possible,
// for example the error might happen in a background goroutine.
// In those cases some logging is necessary,
// but those should be kept at minimal,
// and the library code should provide control to the caller on how to do those
// logging.
//
// This interface is meant to solve Principle 2 above.
// In reality we might actually use different logging libraries in different
// services, and they are not always compatible with each other.
// Wrapper is a simple common ground that it's easy to wrap whatever logging
// library we use into.
//
// With that in mind, this interface should only be used by library code,
// when the case satisfies all of the following 3 criteria:
//
// 1. A bad thing happened.
//
// 2. This is unexpected.
// For expected errors,
// the library should either handle it by itself (e.g. retry),
// or communicate it back to the caller and let them handle it.
//
// 3. This is also recoverable.
// Unrecoverable errors should also be communicated back to the caller to handle.
//
// Baseplate services should use direct logging functions for their logging
// needs, instead of using Wrapper interface.
//
// For production code using baseplate libraries,
// Baseplate services should use ErrorWithSentryWrapper in most cases,
// as whenever the Wrapper is called that's something bad and unexpected
// happened and the service owner should be aware of.
// Non-Baseplate services should use error level in whatever logging library
// they use.
//
// For unit tests of library code using Wrapper,
// TestWrapper is provided that would fail the test when Wrapper is called.
//
// Additionally,
// this interface is also compatible with thrift.Logger and can be used
// interchangeably (sometimes a typecasting is needed).
type Wrapper func(msg string)

// Log is the nil-safe way of calling a log.Wrapper.
func (w Wrapper) Log(msg string) {
	if w != nil {
		w(msg)
	}
}

// NopWrapper is a Wrapper implementation that does nothing.
//
// In most cases you don't need to use it directly.
// The zero value of log.Wrapper is essentially a NopWrapper.
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

// ErrorWithSentryWrapper is a Wrapper implementation that both use Zap logger
// to log at error level, and also send the message to Sentry.
//
// In most cases this should be the one used to pass into Baseplate.go libraries
// expecting a log.Wrapper.
// If the service didn't configure sentry,
// then this wrapper is essentially the same as log.ZapWrapper(log.ErrorLevel).
//
// Note that this should not be used as the logger set into thrift.TSimpleServer,
// as that would use the logger to log network I/O errors,
// which would be too spammy to be sent to Sentry.
// For this reason, it's returning a Wrapper instead of being a Wrapper itself,
// thus forcing an extra typecasting to be used as a thrift.Logger.
func ErrorWithSentryWrapper() Wrapper {
	return func(msg string) {
		Error(msg)
		sentry.CaptureException(errors.New(msg))
	}
}

var (
	_ Wrapper = NopWrapper
)
