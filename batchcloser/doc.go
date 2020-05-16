// Package batchcloser provides an object "BatchCloser" that collects multiple
// io.Closers and closes them all when Closers.Close is called.
//
// It also provides helper methods for wrapping close/cancel functions in
// io.Closer objects.
package batchcloser
