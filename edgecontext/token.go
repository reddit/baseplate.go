package edgecontext

import (
	"github.com/reddit/baseplate.go/timebp"

	jwt "github.com/reddit/jwt-go/v3"
)

// AuthenticationToken defines the json format of the authentication token.
type AuthenticationToken struct {
	jwt.StandardClaims

	// NOTE: Subject field is in StandardClaims.

	Roles []string `json:"roles,omitempty"`

	OAuthClientID   string   `json:"client_id,omitempty"`
	OAuthClientType string   `json:"client_type,omitempty"`
	Scopes          []string `json:"scopes,omitempty"`

	LoID struct {
		ID        string                      `json:"id,omitempty"`
		CreatedAt timebp.TimestampMillisecond `json:"created_ms,omitempty"`
	} `json:"loid,omitempty"`

	suppressor JWTErrorSuppressor
}

// Subject returns the subject field of the token.
func (t AuthenticationToken) Subject() string {
	return t.StandardClaims.Subject
}

// Valid overrides jwt.StandardClaims.Valid.
//
// It checks whether the error should be suppressed,
// and suppress them when appropriate.
//
// By default it behaves the same as jwt.StandardClaims.Valid.
func (t AuthenticationToken) Valid() error {
	err := t.StandardClaims.Valid()
	if t.suppressor != nil && t.suppressor(err) {
		return nil
	}
	return err
}
