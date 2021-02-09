package ecinterface

import (
	"context"

	"github.com/reddit/baseplate.go/secrets"
)

// Interface defines the interface edgecontext implementation must implements.
//
// The string "header" does not necessarily need to be ASCII/UTF-8 string.
// For thrift those headers will be used as-is,
// for HTTP those headers will always be wrapped with additional base64
// encoding.
type Interface interface {
	// HeaderToContext parses the edge context from header,
	// then inject the object into context.
	HeaderToContext(ctx context.Context, header string) (context.Context, error)

	// ContextToHeader extracts edge context object from context,
	// then serializes it into header.
	//
	// It shall return ("", false) when there's no edge context attached to ctx.
	ContextToHeader(ctx context.Context) (header string, ok bool)
}

// FactoryArgs defines the args used in Factory.
type FactoryArgs struct {
	Store *secrets.Store
}

// Factory is the callback used by baseplate.New to create the implementation.
type Factory func(args FactoryArgs) (Interface, error)
