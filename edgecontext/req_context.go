package edgecontext

import (
	"context"
	"sync"

	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/set"
	"github.com/reddit/baseplate.go/thriftbp"

	"github.com/apache/thrift/lib/go/thrift"
)

// An EdgeRequestContext contains context info about an edge request.
type EdgeRequestContext struct {
	// header and raw should always be set during initialization
	header string
	raw    NewArgs

	// token will be validated on first use
	tokenOnce sync.Once
	token     *AuthenticationToken
}

// AuthToken either validates the raw auth token and cache it,
// or return the cached token.
//
// If the validation failed, the error will be logged.
func (e *EdgeRequestContext) AuthToken() *AuthenticationToken {
	e.tokenOnce.Do(func() {
		if token, err := ValidateToken(e.raw.AuthToken); err != nil {
			log.Errorw("token validation failed", "err", err)
			e.token = nil
		} else {
			e.token = token
		}
	})
	return e.token
}

// AttachToContext attaches the header to thrift context.
func (e *EdgeRequestContext) AttachToContext(ctx context.Context) context.Context {
	ctx = thrift.SetHeader(ctx, thriftbp.HeaderEdgeRequest, e.header)
	headers := set.StringSliceToSet(thrift.GetWriteHeaderList(ctx))
	headers.Add(thriftbp.HeaderEdgeRequest)
	ctx = thrift.SetWriteHeaderList(ctx, headers.ToSlice())
	return ctx
}

// SessionID returns the session id of this request.
func (e *EdgeRequestContext) SessionID() string {
	return e.raw.SessionID
}

// User returns the info about the user of this request.
func (e *EdgeRequestContext) User() User {
	return User{
		e: e,
	}
}

// OAuthClient returns the info about the oauth client of this request.
//
// ok will be false if this request does not have a valid auth token.
func (e *EdgeRequestContext) OAuthClient() (client OAuthClient, ok bool) {
	token := e.AuthToken()
	if token == nil {
		return
	}
	return OAuthClient(*token), true
}

// Service returns the info about the client service of this request.
//
// ok will be false if this request does not have a valid auth token.
func (e *EdgeRequestContext) Service() (service Service, ok bool) {
	token := e.AuthToken()
	if token == nil {
		return
	}
	return Service(*token), true
}
