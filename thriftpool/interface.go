package thriftpool

import (
	"io"
)

// Client is a minimal interface for a thrift client needed by the pool.
//
// TTransport interface in thrift satisfies Client interface,
// so embedding the TTransport used by the actual client is a common way to
// implement the ClientOpener.
type Client interface {
	io.Closer

	IsOpen() bool
}

// ClientOpener defines a generator for clients.
type ClientOpener func() (Client, error)

// Pool defines the thrift client pool interface.
type Pool interface {
	io.Closer

	Get() (Client, error)
	Release(c Client) error
	NumActiveClients() int32
	NumAllocated() int32
	IsExhausted() bool
}
