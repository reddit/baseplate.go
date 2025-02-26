package httpbp

import (
	"context"
	"net/http"

	"github.com/reddit/baseplate.go/internal/faults"
)

type clientFaultMiddleware struct {
	injector faults.Injector[*http.Response]
}

// NewClientFaultMiddleware creates and returns a new client-side fault
// injection middleware.
//
// This middleware injects faults into the outgoing HTTP requests based on the
// X-Bp-Fault header values. If valid X-Bp-Fault values exist which are not
// intended to be interpreted by this middleware, then unintended faults could
// be injected. This is extremely unlikely given how specific the headers and
// values must be for them to be compatible, but it's worth calling out
// as an edge case.
func NewClientFaultMiddleware(clientName string) clientFaultMiddleware {
	return clientFaultMiddleware{
		injector: *faults.NewInjector[*http.Response](
			clientName,
			"httpbp.clientFaultMiddleware",
			400,
			599,
		),
	}
}

type httpHeaders struct {
	req *http.Request
}

var _ faults.Headers = httpHeaders{}

// Lookup returns the values of the header, if found.
func (h httpHeaders) LookupValues(ctx context.Context, key string) ([]string, error) {
	return h.req.Header.Values(key), nil
}
