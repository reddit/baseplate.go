package httpbp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"

	httpgk "github.com/go-kit/kit/transport/http"
)

const (
	// ContentTypeHeader is the 'Content-Type' header key.
	ContentTypeHeader = "Content-Type"

	// JSONContentType is the Content-Type header for JSON responses.
	JSONContentType = "application/json; charset=utf-8"

	// HTMLContentType is the Content-Type header for HTML responses.
	HTMLContentType = "text/html; charset=utf-8"
)

// HTTPError is a specialized error that is returned be the Err method specified
// in the ErrorResponse interface.
type HTTPError struct {
	// Code is the status code to set on the HTTP response.  Defaults to 500 if
	// it is not set.
	Code int

	// Message is an optional message that can be returned to the
	// client.  Defaults to the native http.StatusText message for the
	// StatusCode() of the HTTPError.
	Message string

	// Cause is an optional error that can be used to retain the error that
	// led to us returning an HTTP error to the client.
	Cause error
}

// Error returns the standard error string, this is not returned to the client.
func (e HTTPError) Error() string {
	return fmt.Sprintf(
		"httpbp: http error with code %d, message %q and cause %v",
		e.Code,
		e.Message,
		e.Cause,
	)
}

// ResponseMessage returns the error message to send to the client.
func (e HTTPError) ResponseMessage() string {
	if e.Message != "" {
		return e.Message
	}
	return http.StatusText(e.StatusCode())
}

// As implements helper interface for errors.As.
//
// If v is pointer to either HTTPError or *HTTPError, *v will be set into this
// error.
func (e HTTPError) As(v interface{}) bool {
	if target, ok := v.(*HTTPError); ok {
		*target = e
		return true
	} else if target, ok := v.(**HTTPError); ok {
		*target = &e
		return true
	}
	return false
}

// Unwrap implements helper interface for errors.Unwrap.  Returns the optional
// e.Cause error.
func (e HTTPError) Unwrap() error {
	return e.Cause
}

// StatusCode returns the HTTP status code to set on the response.  Defaults to
// 500 if Code is not set on the HTTPError.
func (e HTTPError) StatusCode() int {
	if e.Code == 0 {
		return http.StatusInternalServerError
	}
	return e.Code
}

var (
	_ error = HTTPError{}
	_ error = (*HTTPError)(nil)
)

// ResponseCookies is an interface that your Response objects can implement in
// order to have the httpbp.Encode methods automatically add cookies to the
// response.
type ResponseCookies interface {
	// Return a list of all cookies to set on the response.
	Cookies() []*http.Cookie
}

// ErrorResponse is an interface that your Response objects can implement in
// order to have the httpbp.Encode methods automatically return http errors.
type ErrorResponse interface {
	// Err returns the HTTPError set on the response.
	Err() error
}

type responseEncoder func(w http.ResponseWriter, r interface{}) error

type encodeArgs struct {
	contentType string
	encoder     responseEncoder
}

func encodeResponse(w http.ResponseWriter, response interface{}, args encodeArgs) error {
	w.Header().Set(ContentTypeHeader, args.contentType)

	if resp, ok := response.(httpgk.Headerer); ok {
		for key, values := range resp.Headers() {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
	}

	if resp, ok := response.(ResponseCookies); ok {
		for _, cookie := range resp.Cookies() {
			http.SetCookie(w, cookie)
		}
	}

	if resp, ok := response.(ErrorResponse); ok {
		if respErr := resp.Err(); respErr != nil {
			var he HTTPError
			if !errors.As(respErr, &he) {
				he.Code = 500
				he.Cause = respErr
			}
			http.Error(w, he.ResponseMessage(), he.StatusCode())
			return nil
		}
	}

	// The response code in the HTTPError returned by ErrorResponse.Err takes
	// precedent over this one, so we put this after the error check so we
	// don't even bother to run this if we are returning an error.
	if resp, ok := response.(httpgk.StatusCoder); ok {
		if resp.StatusCode() != 0 && resp.StatusCode() != http.StatusOK {
			w.WriteHeader(resp.StatusCode())
		}
	}

	return args.encoder(w, response)
}

// EncodeJSONResponse implements go-kit's http.EncodeResponseFunc interface and
// encodes the given response as json.
//
// If the response implements go-kit's http.Headerer interface, then the headers
// will be applied to the response, after the Content-Type header is set.
//
// If the response implements the ResponseCookie interface, then any cookies
// returned will be applied to the response, after the headers are set.
//
// If the response implements the ErrorResponse interface, then an error
// response will be returned if Err() is non-nil.  You can use the HTTPError
// object to customize the error response.
//
// If the response implements go-kit's http.StatusCoder interface, then the
// status code returned will be used rather than 200.  If a response implements
// this but returns the default integer value of 0, then the code will still be
// set to 200.  If the response alse implements the ErrorResponse interface,
// then this status code is ignored in favor of the error status code.
func EncodeJSONResponse(_ context.Context, w http.ResponseWriter, response interface{}) error {
	return encodeResponse(w, response, encodeArgs{
		contentType: JSONContentType,
		encoder: func(w http.ResponseWriter, r interface{}) error {
			return json.NewEncoder(w).Encode(r)
		},
	})
}

// EncodeTemplatedResponse encodes the given response as text/html with the
// given template.
//
// This method does not implement the go-kit http.EncodeResponseFunc interface,
// if you want to use this with go-kit, use BuildEncodeTemplatedResponse to
// return a function that wraps EncodeTemplatedResponse with the a single
// template and does implement the http.EncodeResponseFunc interface.
//
// If the response implements go-kit's http.Headerer interface, then the headers
// will be applied to the response, after the Content-Type header is set.
//
// If the response implements the ResponseCookie interface, then any cookies
// returned will be applied to the response, after the headers are set.
//
// If the response implements the ErrorResponse interface, then an error
// response will be returned if Err() is non-nil.  You can use the HTTPError
// object to customize the error response.
//
// If the response implements go-kit's http.StatusCoder interface, then the
// status code returned will be used rather than 200.  If a response implements
// this but returns the default integer value of 0, then the code will still be
// set to 200.  If the response alse implements the ErrorResponse interface,
// then this status code is ignored in favor of the error status code.
func EncodeTemplatedResponse(_ context.Context, w http.ResponseWriter, response interface{}, t *template.Template) error {
	return encodeResponse(w, response, encodeArgs{
		contentType: HTMLContentType,
		encoder: func(w http.ResponseWriter, r interface{}) error {
			return t.Execute(w, r)
		},
	})
}

// BuildEncodeTemplatedResponse returns a function that implements go-kits
// http.EncodeResponseFunc interface and wraps EncodeTemplatedResponse with the
// template passed in.
func BuildEncodeTemplatedResponse(t *template.Template) httpgk.EncodeResponseFunc {
	return func(ctx context.Context, w http.ResponseWriter, response interface{}) error {
		return EncodeTemplatedResponse(ctx, w, response, t)
	}
}

var (
	_ httpgk.EncodeResponseFunc = EncodeJSONResponse
)
