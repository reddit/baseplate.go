package httpbp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strings"
)

const (
	// ContentTypeHeader is the 'Content-Type' header key.
	ContentTypeHeader = "Content-Type"

	// JSONContentType is the Content-Type header for JSON responses.
	JSONContentType = "application/json; charset=utf-8"

	// HTMLContentType is the Content-Type header for HTML responses.
	HTMLContentType = "text/html; charset=utf-8"

	// PlainTextContentType is the Content-Type header for plain text responses.
	PlainTextContentType = "text/plain; charset=utf-8"
)

// ContentWriter is responsible writing the response body and communicating the
// "Content-Type" of the response body.
type ContentWriter interface {
	// ContentType returns the value to set on the "Content-Type" header of the
	// response.
	ContentType() string

	// WriteResponse takes the given response body and writes it to the given
	// writer.
	WriteResponse(w io.Writer, v interface{}) error
}

// ContentWriterFactory is the interface used by the handler to create new
// ContentWriters when serving requests.
type ContentWriterFactory func() ContentWriter

// Response is an HTTP response that can be returned by a baseplate HTTP handler.
//
// Response is responsible for setting the values that are independant of the
// body as well as the ContentWriter to write the response body.
type Response interface {
	// SetCode sets the status code to return with the response.
	SetCode(code int)

	// StatusCode returns the current status code set on the response.
	StatusCode() int

	// ClearCookies clears all cookies currently set on the response.
	ClearCookies()

	// AddCookie adds the cookie to the list of cookies to set on the response.
	AddCookie(cookie *http.Cookie)

	// Cookies returns the list of cookies current set on the response.
	Cookies() []*http.Cookie

	// Headers returns the list of headers to return to the client.
	Headers() http.Header

	// NewHTTPError returns a new HTTPError with the given values using the
	// a new ContentWriter of the same type as the Response.
	//
	// The new HTTPError will not inherit headers or cookies from the Response
	// used to create it.
	NewHTTPError(code int, body interface{}, cause error) HTTPError

	// SetContentWriter replaces the current ContentWriter with the one passed
	// in.
	//
	// A response should be initialized with a default ContentWriter, this is
	// provided so services that want to be able to serve multiple content types
	// from the same endpoint.
	SetContentWriter(w ContentWriter)

	// ContentWriter returns the ContentWriter used to write the response.
	ContentWriter() ContentWriter
}

// HTTPError is an error that implements Response and can be returned by an
// HTTPHandler to return a customized error Response.
type HTTPError interface {
	Response
	error

	// Body is the body value to be passed to Response.WriteResponse for the
	// error.
	Body() interface{}

	// Unwrap implements helper interface for errors.Unwrap.  Should return the
	// internal error that triggered the HTTPError to be returned to the caller.
	Unwrap() error
}

// JSONContentWriter is a ContentWriterFactory that returns a ContentWriter
// for writing JSON.
//
// When using a JSON content writer, your handler should return a value that
// can be marshalled into JSON.  This can either be a struct that defines JSON
// reflection tags or a `map` of values that can be Marshalled to JSON.
func JSONContentWriter() ContentWriter {
	return contentWriter{
		contentType: JSONContentType,
		write: func(w io.Writer, body interface{}) error {
			return json.NewEncoder(w).Encode(body)
		},
	}
}

// HTMLBody is the interface that is expected by an HTML ContentWriter.
type HTMLBody interface {
	// TemplateName returns the name of the template to use to render the HTML
	// response.
	TemplateName() string
}

// BaseHTMLBody can be embedded in another struct to allow that struct to fufill
// the HTMLBody interface.
type BaseHTMLBody struct {
	Name string
}

// TemplateName returns the name of the template to use to render the HTML
// response.
func (b BaseHTMLBody) TemplateName() string {
	return b.Name
}

// HTMLContentWriterFactory returns a ContentWriterFactory that returns a
// ContentWriter for writing HTML using the given template.
//
// When using an HTML content writer, your handler should return a struct that
// implements the HTMLBody interface and can be given as input to t.Execute.
func HTMLContentWriterFactory(templates *template.Template) ContentWriterFactory {
	return func() ContentWriter {
		return contentWriter{
			contentType: HTMLContentType,
			write: func(w io.Writer, body interface{}) error {
				var htmlBody HTMLBody
				var ok bool
				if htmlBody, ok = body.(HTMLBody); !ok {
					return errors.New("httpbp: wrong response type for html response")
				}

				var t *template.Template
				if t = templates.Lookup(htmlBody.TemplateName()); t == nil {
					return fmt.Errorf("httpbp: no html template with name %s", htmlBody.TemplateName())
				}

				return t.Execute(w, htmlBody)
			},
		}
	}
}

// RawContentWriterFactory returns a ContentWriterFactory that returns a
// ContentWriter for writing raw content in the given Content-Type.
//
// When using a raw content writer, your handler should return an object that
// implements `io.Reader`, a string, or a byte slice.
func RawContentWriterFactory(contentType string) ContentWriterFactory {
	return func() ContentWriter {
		return contentWriter{
			contentType: contentType,
			write: func(w io.Writer, body interface{}) error {
				var r io.Reader
				switch b := body.(type) {
				default:
					return fmt.Errorf("httpbp: %#v is not an io.Reader", body)
				case io.Reader:
					r = b
				case string:
					r = strings.NewReader(b)
				case []byte:
					r = bytes.NewReader(b)
				}
				_, err := io.Copy(w, r)
				return err
			},
		}
	}
}

// NewResponse returns a new Response object with a ContentWriter built by the
// given ContentWriterFactory.
//
// NewResponse is provided for testing purposes and should not be used directly
// as the http.Handler given by httpbp.NewHandler provides your HandlerFunc with
// an already initialized response.
func NewResponse(contentFactory ContentWriterFactory) Response {
	return &httpResponse{
		headers:       make(http.Header),
		content:       contentFactory(),
		writerFactory: contentFactory,
	}
}

type contentWriter struct {
	contentType string
	write       func(io.Writer, interface{}) error
}

func (c contentWriter) ContentType() string {
	return c.contentType
}

func (c contentWriter) WriteResponse(w io.Writer, v interface{}) error {
	return c.write(w, v)
}

type httpResponse struct {
	code          int
	headers       http.Header
	cookies       []*http.Cookie
	content       ContentWriter
	writerFactory ContentWriterFactory

	err HTTPError
}

func (r *httpResponse) SetCode(code int) {
	r.code = code
}

func (r httpResponse) StatusCode() int {
	return r.code
}

func (r httpResponse) Headers() http.Header {
	return r.headers
}

func (r httpResponse) Cookies() []*http.Cookie {
	cookies := make([]*http.Cookie, len(r.cookies))
	copy(cookies, r.cookies)
	return cookies
}

func (r *httpResponse) AddCookie(cookie *http.Cookie) {
	r.cookies = append(r.cookies, cookie)
}

func (r *httpResponse) ClearCookies() {
	r.cookies = nil
}

func (r *httpResponse) SetContentWriter(w ContentWriter) {
	r.content = w
}

func (r httpResponse) ContentWriter() ContentWriter {
	return r.content
}

func (r httpResponse) NewHTTPError(code int, body interface{}, cause error) HTTPError {
	return NewHTTPError(code, body, cause, r.writerFactory)
}

// NewHTTPError returns a new HTTPError object initialized with the given
// values.
//
// NewHTTPError is provided for testing purposes and should not be used directly,
// you should use Request.NewHTTPError to create HTTP errors rather than
// creating them directly using NewHTTPError.
func NewHTTPError(code int, body interface{}, cause error, writerFactory ContentWriterFactory) HTTPError {
	resp := NewResponse(writerFactory)
	resp.SetCode(code)
	return &httpError{
		Response: resp,
		body:     body,
		cause:    cause,
	}
}

type httpError struct {
	Response

	body  interface{}
	cause error
}

func (e httpError) Body() interface{} {
	return e.body
}

func (e httpError) Error() string {
	return fmt.Sprintf(
		"httpbp: http error with code %d and cause %v",
		e.StatusCode(),
		e.Unwrap(),
	)
}

func (e httpError) Unwrap() error {
	return e.cause
}

var (
	_ HTTPError = httpError{}
	_ HTTPError = (*httpError)(nil)
)
