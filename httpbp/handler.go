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
// the given Middleware.
//
// Most services should not use NewHander and should use a BaseplateHandlerFactory
// instead.
// NewHander is provided for those who need to avoid the default Baseplate
// middleware or for testing purposes.
func NewHandler(handle HandlerFunc, middlewares ...Middleware) http.Handler {
	return handler{handle: Wrap(handle, middlewares...)}
}

// BaseplateHandlerFactory can be used to create multiple BaseplateHandlers,
// HandlerFuncs wrapped with the default Baseplate middleware as well as any
// additional middleware supplied.
type BaseplateHandlerFactory struct {
	// Args is the arguments for the default baseplate Middleware that will be
	// supplide to each handler.
	Args DefaultMiddlewareArgs

	// Middlewares are middleware that will wrap all HandlerFuncs created by the
	// BaseplateHandlerFactory.  These are applied after the default Baseplate
	// Middlewares and before the per-handler Middleware passed to NewHandler.
	Middlewares []Middleware
}

// NewHandler returns a new HandlerFunc that is the result of wrapping `handle`
// with the default Baseplate Middleware, the list of Middleware given to the
// factory, and any Middleware passed in.
//
// Middlewares are applied in the following order:
//		1. httpbp.DefaultMiddleware()
//		2. BaseplateHandlerFactory.Middlewares
//		3. Additional, per-handler middleware passed into
//		   BaseplateHandlerFactory.NewHandler
func (f BaseplateHandlerFactory) NewHandler(name string, handle HandlerFunc, middlewares ...Middleware) http.Handler {
	defaults := DefaultMiddleware(name, f.Args)
	wrappers := make([]Middleware, 0, len(defaults)+len(f.Middlewares)+len(middlewares))
	wrappers = append(wrappers, defaults...)
	wrappers = append(wrappers, f.Middlewares...)
	wrappers = append(wrappers, middlewares...)
	return NewHandler(handle, wrappers...)
}
