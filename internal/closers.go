package internal

import "io"

type anonymousCloser struct {
	io.Closer
	closeFunc func() error
}

// NewAnonymousCloser is a convenience function to transform anonymous functions into
// io.Closers
func NewAnonymousCloser(f func() error) io.Closer {
	return &anonymousCloser{closeFunc: f}
}

// NoOpCloser is the no-op version of an io.Closer
var NoOpCloser io.Closer = NewAnonymousCloser(func() error { return nil })
