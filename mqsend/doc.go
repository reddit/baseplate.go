// Package mqsend is a pure go implementation of posix message queue for Linux,
// using syscalls.
//
// The purpose of this package is to provide a pure go (no cgo) implementation
// on 64-bit Linux systems to be able to send messages to the posix message
// queue, which will be consumed by sidecars implemented in baseplate.py.
// It's never meant to be a complete implementation of message queue features.
// It does NOT have supports for:
//
// - Non-linux systems (e.g. macOS)
// - Non-64-bit systems (e.g. 32-bit Linux)
// - Non-send operations (e.g. receive)
//
// If you need those features, this is not the package for you.
package mqsend
