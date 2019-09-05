// Package log provides common logging interface used by other baseplate
// packages.
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
