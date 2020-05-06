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
//
// To use a ContentWriter, pass it to httpbp.WriteResponse rather than using
// it directly.
type ContentWriter interface {
	// ContentType returns the value to set on the "Content-Type" header of the
	// response.
	ContentType() string

	// WriteBody takes the given response body and writes it to the given
	// writer.
	WriteBody(w io.Writer, v interface{}) error
}

// Response is the non-header content to be written in an HTTP response.
type Response struct {
	// Body is the response body to write using a ContentWriter.  You should
	// ensure that Body is something that can be successfully written by the
	// ContentWriter, otherwise an error will be returned instead.
	Body interface{}

	// Code is the status code to set on the response, this is optional and only
	// should be set if you want to return something other than http.StatusOK (200).
	Code int
}

// WriteResponse writes the given Response to the given ResponseWriter using the
// given ContentWriter.  It also sets the Content-Type header on the response to
// the one defined by the ContentWriter and sets the status code of the response
// if set on the Response object.
//
// WriteResponse generally does not need to be called directly, instead you can
// use one of the helper methods to call it with a pre-defined ContentWriter.
func WriteResponse(w http.ResponseWriter, cw ContentWriter, resp Response) error {
	w.Header().Set(ContentTypeHeader, cw.ContentType())
	if resp.Code > 0 {
		w.WriteHeader(resp.Code)
	}
	return cw.WriteBody(w, resp.Body)
}

// WriteJSON calls WriteResponse with a JSON ContentWriter.
func WriteJSON(w http.ResponseWriter, resp Response) error {
	return WriteResponse(w, JSONContentWriter(), resp)
}

// WriteHTML calls WriteResponse with an HTML ContentWriter using the given
// templates.
func WriteHTML(w http.ResponseWriter, resp Response, templates *template.Template) error {
	return WriteResponse(w, HTMLContentWriter(templates), resp)
}

// WriteRawContent calls WriteResponse with a Raw ContentWriter with the given
// Content-Type.
func WriteRawContent(w http.ResponseWriter, resp Response, contentType string) error {
	return WriteResponse(w, RawContentWriter(contentType), resp)
}

// JSONContentWriter returns a ContentWriter for writing JSON.
//
// When using a JSON ContentWriter, your Response.Body should be a value that
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

// HTMLContentWriter returns a ContentWriter for writing HTML using the given
// templates.
//
// When using an HTML ContentWriter, your Response.Body should be an object that
// implements the HTMLBody interface and can be given as input to  t.Execute.
// If it does not, an error will be returned.  An error will also be returned if
// there is no template available with the TemplateName() returned by Response.Body.
func HTMLContentWriter(templates *template.Template) ContentWriter {
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

// RawContentWriter returns a ContentWriter for writing raw content with the
// given Content-Type.
//
// When using a raw content writer, your your Response.Body should be an object
// that  implements one of the io.Reader or fmt.Stringer interfaces, a string,
// or a byte slice.  If it is not one of these, an error will be returned.
func RawContentWriter(contentType string) ContentWriter {
	return contentWriter{
		contentType: contentType,
		write: func(w io.Writer, body interface{}) error {
			var r io.Reader
			switch b := body.(type) {
			default:
				return fmt.Errorf("httpbp: %#v is not an io.Reader", body)
			case io.Reader:
				r = b
			case fmt.Stringer:
				r = strings.NewReader(b.String())
			case string:
				r = strings.NewReader(b)
			case []byte:
				r = bytes.NewReader(b)
			case nil:
				r = strings.NewReader("")
			}
			_, err := io.Copy(w, r)
			return err
		},
	}
}

type contentWriter struct {
	contentType string
	write       func(io.Writer, interface{}) error
}

func (c contentWriter) ContentType() string {
	return c.contentType
}

func (c contentWriter) WriteBody(w io.Writer, v interface{}) error {
	return c.write(w, v)
}
