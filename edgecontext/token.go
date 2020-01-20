package edgecontext

import (
	"github.com/reddit/baseplate.go/timebp"

	jwt "gopkg.in/dgrijalva/jwt-go.v3"
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
}

// Subject returns the subject field of the token.
func (t AuthenticationToken) Subject() string {
	return t.StandardClaims.Subject
}
