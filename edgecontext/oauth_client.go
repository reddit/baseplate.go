package edgecontext

import (
	"strings"
)

// An OAuthClient wrapps AuthenticationToken and provides info about a client
// using OAuth2.
type OAuthClient AuthenticationToken

// ID returns the OAuth client id.
func (o OAuthClient) ID() string {
	return AuthenticationToken(o).OAuthClientID
}

// IsType checks if the given OAuth client matches any of the given types.
//
// When checking the type of the current OAuthClient,
// you should check that the type "is" one of the allowed types,
// rather than checking that it "is not" a disallowed type.
//
// For example, use:
//
//     if client.IsType("third_party")
//
// Instead of:
//
//     if !client.IsType("first_party")
func (o OAuthClient) IsType(types ...string) bool {
	clientType := AuthenticationToken(o).OAuthClientType
	for _, t := range types {
		if clientType == strings.ToLower(t) {
			return true
		}
	}
	return false
}
