package edgecontext

import (
	"strings"
	"time"

	"github.com/reddit/baseplate.go/experiments"
)

const userPrefix = "t2_"

// An User wraps *EdgeRequestContext and provides info about a logged in or
// logged our user.
type User struct {
	e *EdgeRequestContext
}

// ID returns the authenticated account id of the user.
//
// ok will be false if the user is not logged in.
func (u User) ID() (id string, ok bool) {
	token := u.e.AuthToken()
	if token == nil {
		return
	}
	if strings.HasPrefix(token.Subject(), userPrefix) {
		return token.Subject(), true
	}
	return
}

// IsLoggedIn returns true if the user is logged in.
func (u User) IsLoggedIn() bool {
	_, ok := u.ID()
	return ok
}

// LoID returns the LoID of this user.
func (u User) LoID() (loid string, ok bool) {
	if u.e.raw.LoID != "" {
		return u.e.raw.LoID, true
	}
	token := u.e.AuthToken()
	if token == nil {
		return
	}
	return token.LoID.ID, token.LoID.ID != ""
}

// CookieCreatedAt returns the time the cookie was created.
func (u User) CookieCreatedAt() (ts time.Time, ok bool) {
	if !u.e.raw.LoIDCreatedAt.IsZero() {
		return u.e.raw.LoIDCreatedAt, true
	}
	token := u.e.AuthToken()
	if token == nil {
		return
	}
	ts = token.LoID.CreatedAt.ToTime()
	return ts, !ts.IsZero()
}

// Roles returns the roles the user has.
func (u User) Roles() []string {
	token := u.e.AuthToken()
	if token == nil {
		return nil
	}
	return token.Roles
}

// HasRole returns true if the user has the specific role.
func (u User) HasRole(role string) bool {
	token := u.e.AuthToken()
	if token == nil {
		return false
	}
	// Since in most cases the roles slice would be quite small,
	// it's better to iterate them than converting the slice into a set.
	for _, r := range token.Roles {
		if strings.ToLower(role) == strings.ToLower(r) {
			return true
		}
	}
	return false
}

// UpdateExperimentEvent updates the passed in experiment event with user info.
//
// It always updates UserID, LoggedIn, and CookieCreatedAt fields and never
// touches other fields.
func (u User) UpdateExperimentEvent(ee *experiments.ExperimentEvent) {
	var loggedIn bool
	ee.UserID, loggedIn = u.ID()
	if !loggedIn {
		ee.UserID, _ = u.LoID()
	}
	ee.LoggedIn = &loggedIn
	ee.CookieCreatedAt, _ = u.CookieCreatedAt()
}
