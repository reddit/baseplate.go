package clientpool

import (
	"io"
)

// Client is a minimal interface for a client needed by the pool.
//
// TTransport interface in thrift satisfies Client interface,
// so embedding the TTransport used by the actual client is a common way to
// implement the ClientOpener for thrift Clients.
// thriftclient.TTLClient also implements it, with an additional TTL to
// the transport.
type Client interface {
	io.Closer

	IsOpen() bool
}

// ClientOpener defines a generator for clients.
type ClientOpener func() (Client, error)

// Pool defines thrift client pool interface.
type Pool interface {
	io.Closer

	Get() (Client, error)
	Release(c Client) error
	NumActiveClients() int32
	NumAllocated() int32
	IsExhausted() bool
}
