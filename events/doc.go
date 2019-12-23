// Package events implements event publisher per baseplate spec.
//
// This package is mainly just the serialization part.
// The actual publishing part is handled by sidecar implemented by baseplate.py,
// and communicated via mqsend package.
package events
