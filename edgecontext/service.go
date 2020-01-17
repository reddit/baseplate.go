package edgecontext

import (
	"strings"
)

const servicePrefix = "service/"

// A Service wraps AuthenticationToken and provides info about an authenticated
// service talking to us.
type Service AuthenticationToken

// Name returns the name of the service.
//
// If it's not coming from an authenticated service,
// ("", false) will be returned.
func (s Service) Name() (name string, ok bool) {
	subject := AuthenticationToken(s).Subject()
	if strings.HasPrefix(subject, servicePrefix) {
		return subject[len(servicePrefix):], true
	}
	return
}
