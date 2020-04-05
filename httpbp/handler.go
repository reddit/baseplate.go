package httpbp

import (
	"context"
	"errors"
	"html/template"
	"net/http"
)

// HandlerFunc handles a single HTTP request and can be wrapped in Middleware.
//
// The base context is extracted from the http.Request and should be used rather
// than the context in http.Request.  This is provided for conveinence and
// consistency accross Baseplate.
type HandlerFunc func(context.Context, *http.Request, Response) (interface{}, error)

func abort(w http.ResponseWriter) {
	code := http.StatusInternalServerError
	http.Error(w, http.StatusText(code), code)
}

func writeResponse(w http.ResponseWriter, resp Response, body interface{}) error {
	for key, values := range resp.Headers() {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	for _, cookie := range resp.Cookies() {
		http.SetCookie(w, cookie)
	}

	cw := resp.ContentWriter()
	w.Header().Set(ContentTypeHeader, cw.ContentType())

	if resp.StatusCode() != 0 && resp.StatusCode() != http.StatusOK {
		w.WriteHeader(resp.StatusCode())
	}
	return cw.WriteResponse(w, body)
}

type handler struct {
	handle      HandlerFunc
	contentFact ContentWriterFactory
}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	resp := NewResponse(h.contentFact)
	body, err := h.handle(ctx, r, resp)
	if err != nil {
		var httpErr HTTPError
		if errors.As(err, &httpErr) {
			resp = httpErr
			body = httpErr.Body()
		} else {
			abort(w)
			return
		}
	}

	if err = writeResponse(w, resp, body); err != nil {
		abort(w)
	}
}

var (
	_ http.Handler = handler{}
	_ http.Handler = (*handler)(nil)
)

// NewHandler returns a new http.Handler using the given ContentWriterFactory to
// initialize Response objects, with the given HandlerFunc wrapped with the
// given Middleware.
//
// Most services should not use NewHander and should use NewBaseplateHandler
// instead.
// NewHander is provided for those who need to avoid the default Baseplate
// middleware or for testing purposes.
func NewHandler(handle HandlerFunc, factory ContentWriterFactory, middlewares ...Middleware) http.Handler {
	return handler{
		handle:      Wrap(handle, middlewares...),
		contentFact: factory,
	}
}

// NewBaseplateHandler returns a new http.Handler using the given
// ContentWriterFactory to initialize Response objects, with the given
// HandlerFunc wrapped with first the default Baseplate Middleware and then the
// given Middleware.
func NewBaseplateHandler(
	handle HandlerFunc,
	factory ContentWriterFactory,
	args DefaultMiddlewareArgs,
	middlewares ...Middleware,
) http.Handler {
	middlewares = append(
		DefaultMiddleware(args),
		middlewares...,
	)
	return NewHandler(handle, factory, middlewares...)
}

// NewBaseplateJSONHandler calls NewBaseplateHandler using JSONContentWriter as
// the ContentWriterFactory.
func NewBaseplateJSONHandler(handle HandlerFunc, args DefaultMiddlewareArgs, middlewares ...Middleware) http.Handler {
	return NewBaseplateHandler(handle, JSONContentWriter, args, middlewares...)
}

// NewBaseplateHTMLHandler calls NewBaseplateHandler using
// HTMLContentWriterFactory(t) as the ContentWriterFactory.
func NewBaseplateHTMLHandler(handle HandlerFunc, t *template.Template, args DefaultMiddlewareArgs, middlewares ...Middleware) http.Handler {
	return NewBaseplateHandler(handle, HTMLContentWriterFactory(t), args, middlewares...)
}

// NewBaseplateRawHandler calls NewBaseplateHandler using
// RawContentWriterFactory(contentType) as the ContentWriterFactory.
func NewBaseplateRawHandler(handle HandlerFunc, contentType string, args DefaultMiddlewareArgs, middlewares ...Middleware) http.Handler {
	return NewBaseplateHandler(handle, RawContentWriterFactory(contentType), args, middlewares...)
}
