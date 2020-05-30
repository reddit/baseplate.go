package edgecontext

import (
	"errors"
	"fmt"
	"sync"

	"github.com/gofrs/uuid"

	"github.com/reddit/baseplate.go/experiments"
)

// An EdgeRequestContext contains context info about an edge request.
type EdgeRequestContext struct {
	impl *Impl

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
		if token, err := e.impl.ValidateToken(e.raw.AuthToken); err != nil {
			// empty jwt token is considered "normal", no need to spam them in logs.
			if !errors.Is(err, ErrEmptyToken) {
				e.impl.logger.Log("token validation failed: " + err.Error())
			}
			e.token = nil
		} else {
			e.token = token
		}
	})
	return e.token
}

// Header returns the raw, underlying edge request context header that was
// parsed to create the EdgeRequestContext object.
//
// This is not really intended to be used directly but to allow us to propogate
// the header between services.
func (e *EdgeRequestContext) Header() string {
	return e.header
}

// SessionID returns the session id of this request.
func (e *EdgeRequestContext) SessionID() string {
	return e.raw.SessionID
}

// DeviceID returns the device id of this request.
func (e *EdgeRequestContext) DeviceID() string {
	return e.raw.DeviceID
}

// User returns the info about the user of this request.
func (e *EdgeRequestContext) User() User {
	return User{
		e: e,
	}
}

// OriginService returns the info about the origin of this request.
func (e *EdgeRequestContext) OriginService() OriginService {
	return OriginService{
		raw: e.raw,
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

// UpdateExperimentEvent updates the passed in experiment event with info from
// this edge request context.
//
// It always updates UserID, LoggedIn, CookieCreatedAt, OAuthClientID,
// SessionID, and DeviceID fields,
// and never touches other fields in experiment event.
//
// The caller should create an experiments.ExperimentEvent object,
// with other non-edge-request related fields already filled,
// call this function to update edge-request related fields updated,
// then pass it to an event logger.
func (e *EdgeRequestContext) UpdateExperimentEvent(ee *experiments.ExperimentEvent) {
	e.User().UpdateExperimentEvent(ee)
	if client, ok := e.OAuthClient(); ok {
		client.UpdateExperimentEvent(ee)
	} else {
		ee.OAuthClientID = ""
	}
	ee.SessionID = e.SessionID()
	if deviceID := e.DeviceID(); deviceID != "" {
		var err error
		ee.DeviceID, err = uuid.FromString(deviceID)
		if err != nil {
			ee.DeviceID = uuid.Nil
			e.impl.logger.Log(fmt.Sprintf(
				"Failed to parse device id %q into uuid: %v",
				deviceID,
				err,
			))
		}
	} else {
		ee.DeviceID = uuid.Nil
	}
}

// OriginService holds metadata about the origin of the request.
type OriginService struct {
	raw NewArgs
}

// Name returns the name of the service that serves as the origin of the request.
func (os OriginService) Name() string {
	return os.raw.OriginServiceName
}
