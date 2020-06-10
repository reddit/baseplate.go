package httpbp

import (
	"context"
	"errors"
	"net/http"

	"github.com/reddit/baseplate.go/log"
)

// HandlerFunc handles a single HTTP request and can be wrapped in Middleware.
//
// The base context is extracted from the http.Request and should be used rather
// than the context in http.Request.  This is provided for conveinence and
// consistency across Baseplate.
//
// HandlerFuncs are free to write directly to the given ResponseWriter but
// WriteResponse and its helpers for common Content-Types have been provided to
// simplify writing responses (including status code) so you should not need to.
// Headers and cookies should still be set using the ResponseWriter.
//
// If a HandlerFunc returns an error, the Baseplate implementation of
// http.Handler will attempt to write an error response, so you should generally
// avoid writing your response until the end of your handler call so you know
// there are not any errors.  If you return an HTTPError, it will use that to
// return a custom error response, otherwise it returns a generic, plain-text
// http.StatusInternalServerError (500) error message.
type HandlerFunc func(context.Context, http.ResponseWriter, *http.Request) error

type handler struct {
	handle HandlerFunc
}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := h.handle(ctx, w, r); err != nil {
		var httpErr HTTPError
		if errors.As(err, &httpErr) {
			err = WriteResponse(w, httpErr.ContentWriter(), httpErr.Response())
			if err == nil {
				return
			}
		}
		log.Error("Unhandled server error: " + err.Error())
		code := http.StatusInternalServerError
		http.Error(w, http.StatusText(code), code)
	}
}

var (
	_ http.Handler = handler{}
	_ http.Handler = (*handler)(nil)
)

// NewHandler returns a new http.Handler with the given HandlerFunc wrapped with
// the given Middleware. The given "name" will be passed to all of the middlewares.
//
// Most services should not use NewHander and should use NewBaseplateServer to
// create an entire Server with all of its handlers instead.
// NewHander is provided for those who need to avoid the default Baseplate
// middleware or for testing purposes.
func NewHandler(name string, handle HandlerFunc, middlewares ...Middleware) http.Handler {
	return handler{handle: Wrap(name, handle, middlewares...)}
}
