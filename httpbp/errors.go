package httpbp

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/reddit/baseplate.go/errorsbp"
)

const (
	// RetryAfterHeader is the standard "Retry-After" header key defined in RFC2616.
	//
	// https://www.w3.org/Protocols/rfc2616/rfc2616-sec14.html
	RetryAfterHeader = "Retry-After"

	// ErrorPageTemplate is the template for the default HTML error response.
	ErrorPageTemplate = `<html>
<head>
<style>
		table {
			border-collapse: collapse;
		}

		th {
			text-align: center;
		}

		td  {
			text-align: left;
		}

		table, th, td {
			border: 1px solid black;
			padding: 5px;
		}
</style>
<body>
	<h2>{{ .Reason }}</h2>
	<p>{{ .Explanation }}</p>
{{ if .Details }}
	<table>
		<tr>
			<th colspan="2">Details</th>
		</tr>
{{ range $k, $v := .Details }}
		<tr>
			<td>{{ $k }}</td>
			<td>{{ $v }}</td>
		</tr>
{{ end }}
	</table>
{{ end }}
</body>
</head>
</html>`

	// DefaultErrorTemplateName is the name for the shared, default HTML template
	// for error responses.
	DefaultErrorTemplateName = "httpbp/error"
)

// Well-known errors for middleware layer.
var (
	// ErrConcurrencyLimit is returned by the max concurrency middleware if
	// there are too many requests in-flight.
	ErrConcurrencyLimit = errors.New("hit concurrency limit")
)

// ClientConfig errors are returned if the configuration validation fails.
var (
	ErrConfigMissingSlug              = errors.New("slug cannot be empty")
	ErrConfigInvalidMaxErrorReadAhead = errors.New("maxErrorReadAhead value needs to be positive")
	ErrConfigInvalidMaxConnections    = errors.New("maxConnections value needs to be positive")
)

// HTTPError is an error that and can be returned by an  HTTPHandler to return a
// customized error response.
type HTTPError interface {
	error

	// Response returns the custom Response for the error to be written by
	// the ContentWriter.
	Response() Response

	// ContentWriter returns the ContentWriter object to use to write the error
	// response.
	ContentWriter() ContentWriter

	// Unwrap implements helper interface for errors.Unwrap.  Should return the
	// internal error that triggered the HTTPError to be returned to the caller.
	Unwrap() error
}

func newHTTPError(code int, body interface{}, cause error, cw ContentWriter) HTTPError {
	return &httpError{
		resp: Response{
			Code: code,
			Body: body,
		},
		cw:    cw,
		cause: cause,
	}
}

type httpError struct {
	resp  Response
	cw    ContentWriter
	cause error
}

func (e httpError) Response() Response {
	return e.resp
}
func (e httpError) ContentWriter() ContentWriter {
	return e.cw
}

func (e httpError) Error() string {
	return fmt.Sprintf(
		"httpbp: http error with code %d and cause %v",
		e.Response().Code,
		e.Unwrap(),
	)
}

func (e httpError) Unwrap() error {
	return e.cause
}

// ErrorResponseJSONWrapper wraps the ErrorResponseBody for JSON responses.
//
// ErrorResponseJSONWrapper should not be used directly, it is used automatically
// by JSONError.  It is exported to provide documentation for the final response
// format.
type ErrorResponseJSONWrapper struct {
	Success bool           `json:"success"`
	Error   *ErrorResponse `json:"error"`
}

// ErrorResponse is the base struct used by all the of standard errors in httpbp.
//
// You should not generally need to create ErrorResponses manually as standard
// ones have been provided for each of the common 4xx and 5xx errors but it is
// made available for custom scenarios.
type ErrorResponse struct {
	// A standard, machine readable string representing the error.  Should be
	// UPPER_SNAKE_CASE.
	//
	// Ex: "INTERNAL_SERVER_ERROR"
	Reason string `json:"reason"`

	// A human readable explanation for the error.
	//
	// Ex: "The server has either erred or is incapable of performing the request."
	Explanation string `json:"explanation"`

	// Optional map of invalid fields to error messages, for use with errors where
	// the client sent invalid input (400).
	//
	// This allows servers to return validation information for multiple
	// fields at once.
	//
	// Ex:
	//	{
	//		"foo.name": "This field is required.",
	//		"bar.id": "This field is required.",
	//	}
	//
	// Details can be set manually or using ErrorResponse.WithDetails.
	// Details are returned to the caller and should be something you are
	// comfortable presenting to an end-user.
	Details map[string]string `json:"details,omitempty"`

	code         int
	raw          string
	templateName string
}

// WithDetails can be used to set the Details on an ErrorResponse in a way that
// can be chained in a call to `HTMLError`.
//
// Ex:
//	return httpbp.JSONError(
//		httpbp.InvalidRequest().WithDetails(map[string]string{
//			"foo": "foo must be > 0",
//			"bar": "bar must be non-nil",
//		})
//		errors.New("validation"),
//	)
//
// This is provided as syntactic sugar and is not required to set Details.
func (r *ErrorResponse) WithDetails(details map[string]string) *ErrorResponse {
	for k, v := range details {
		r.Details[k] = v
	}
	return r
}

// WithRawResponse is used to set the respose to return when using a Raw content
// writer.
//
// This is ignored when using any other content writer.
//
// This returns an ErrorResponse so it can be chained in a call to `HTMLError`:
//
//	return httpbp.RawError(
//		httpbp.BadGateway().WithRawResponse("oops"),
//		errors.New("example"),
//	)
func (r *ErrorResponse) WithRawResponse(raw string) *ErrorResponse {
	r.raw = raw
	return r
}

// WithTemplateName is used to set the name of the HTML template to use when using
// an HTML content writer.
//
// This is ignored when using any other content writer.
// If you are creating a custom template, note that the ErrorResponse struct is
// what is passed to the template to render so you are limited to the values
// exported by ErrorResponse.
//
// This returns an ErrorResponse so it can be chained in a call to `HTMLError`:
//
//	return httpbp.HTMLError(
//		httpbp.BadGateway().WithTemplateName("custom"),
//		errors.New("example"),
//	)
func (r *ErrorResponse) WithTemplateName(name string) *ErrorResponse {
	r.templateName = name
	return r
}

// Retryable communicates to the  caller that the request may be retried after
// the given duration.
//
// This returns an ErrorResponse so it can be chained in a call to any of the
// `Error` methods provided in httpbp.
//
//	return httpbp.JSONError(
//		httpbp.ServiceUnavailable().Retryable(w, time.Hour),
//		errors.New("downtime"),
//	)
func (r *ErrorResponse) Retryable(w http.ResponseWriter, retryAfter time.Duration) *ErrorResponse {
	after := strconv.FormatFloat(float64(retryAfter)/float64(time.Second), 'f', -1, 64)
	w.Header().Set(RetryAfterHeader, after)
	return r
}

// String implements the fmt.Stringer interface which allows ErrorResponse to be
// used as a Raw response.
//
// Returns r.Reason by default, this can be customized by using ErrorResponse.WithRawResponse.
func (r ErrorResponse) String() string {
	if r.raw != "" {
		return r.raw
	}
	return r.Reason
}

// TemplateName implements the httpbp.HTMLBody interface which allows ErrorResponse
// to be used as an HTML response.
//
// Return httpbp.DefaultErrorTemplateName by default, this can be customized by
// using ErrorResponse.WithTemplateName.
func (r ErrorResponse) TemplateName() string {
	if r.templateName != "" {
		return r.templateName
	}
	return DefaultErrorTemplateName
}

// NewErrorResponse returns a new ErrorResponse with the given inputs.
//
// This can be used to create custom NewErrorResponses, but it is encouraged that
// developers use the standard ones provided by httpbp rather than creating them
// directly.
func NewErrorResponse(code int, reason string, explanation string) *ErrorResponse {
	return &ErrorResponse{
		code:        code,
		Reason:      reason,
		Explanation: explanation,
		Details:     make(map[string]string),
	}
}

var (
	_ HTMLBody     = ErrorResponse{}
	_ HTMLBody     = (*ErrorResponse)(nil)
	_ fmt.Stringer = ErrorResponse{}
	_ fmt.Stringer = (*ErrorResponse)(nil)
)

// JSONError returns the given error as an HTTPError that will write JSON.
func JSONError(resp *ErrorResponse, cause error) HTTPError {
	body := ErrorResponseJSONWrapper{Success: false, Error: resp}
	return newHTTPError(resp.code, body, cause, JSONContentWriter())
}

// HTMLError returns the given error as an HTTPError that will write HTML.
func HTMLError(resp *ErrorResponse, cause error, t *template.Template) HTTPError {
	return newHTTPError(resp.code, resp, cause, HTMLContentWriter(t))
}

// RawError returns the given error as an HTTPError that will write a raw
// response of the given content-type.
func RawError(resp *ErrorResponse, cause error, contentType string) HTTPError {
	return newHTTPError(resp.code, resp, cause, RawContentWriter(contentType))
}

// RegisterDefaultErrorTemplate adds the default HTML template for error pages to the
// given templates and returns the result.
//
// This only registeres the single, shared, default template, if you want to
// use custom HTML templates for specific errors, you will need to customize the
// template name on the error by calling `SetTemplateName`
func RegisterDefaultErrorTemplate(t *template.Template) (*template.Template, error) {
	return t.New(DefaultErrorTemplateName).Parse(ErrorPageTemplate)
}

// RegisterCustomDefaultErrorTemplate adds the custom template passed in as the
// default HTML template for error pages, rather than the default template
// provided by baseplate.
//
// If you are creating a custom template, note that the ErrorResponse struct is
// what is passed to the template to render so you are limited to the values
// exported by ErrorResponse.
func RegisterCustomDefaultErrorTemplate(t *template.Template, text string) (*template.Template, error) {
	return t.New(DefaultErrorTemplateName).Parse(text)
}

// BadRequest is for 400 responses.
//
// This is appropriate for when the client sends a malformed or invalid request.
//
// If the client sent an invalid request, you can send the details using the
// Details map in the ErrorResponse.
func BadRequest() *ErrorResponse {
	return NewErrorResponse(
		http.StatusBadRequest,
		"BAD_REQUEST",
		"The request sent was invalid.",
	)
}

// Unauthorized is for 401 responses.
//
// This is appropriate for when you fail to authenticate the request.
//
// It may be appropriate for the client to retry this request in the event, for
// example, if they used an expired authentication credential, they can retry
// after fetching a new one.
func Unauthorized() *ErrorResponse {
	return NewErrorResponse(
		http.StatusUnauthorized,
		"UNAUTHORIZED",
		"This server could not verify that you are authorized to access the document you requested.",
	)
}

// PaymentRequired is for 402 responses.
//
// PaymentRequired is reserved for future use but has no standard around its
// use. It is intended to communicate to the client that their request can not
// be completed until a payment is made.
func PaymentRequired() *ErrorResponse {
	return NewErrorResponse(
		http.StatusPaymentRequired,
		"PAYMENT_REQUIRED",
		"This server cannot process the request until payment is made.",
	)
}

// Forbidden is for 403 responses.
//
// This is appropriate for when you can authenticate a request but the client
// does not have access to the requested resource.
//
// Unlike Unauthorized, refreshing an authentication resource and trying again
// will not make a difference.
func Forbidden() *ErrorResponse {
	return NewErrorResponse(
		http.StatusForbidden,
		"FORBIDDEN",
		"You do not have access to the requested resource.",
	)
}

// NotFound is for 404 responses.
//
// This is appropriate for when the client tries to access something that does
// not exist.
func NotFound() *ErrorResponse {
	return NewErrorResponse(
		http.StatusNotFound,
		"NOT_FOUND",
		"The requested resource could not be found.",
	)
}

// MethodNotAllowed is for 405 responses.
//
// This is appropriate for when the client tries to access a resource that does
// exist using an HTTP method that it does not support.
func MethodNotAllowed() *ErrorResponse {
	return NewErrorResponse(
		http.StatusMethodNotAllowed,
		"NOT_FOUND",
		"The requested resource could not be found.",
	)
}

// Conflict is for 409 responses.
//
// This is appropriate for when a client request would cause a conflict with
// the current state of the server.
func Conflict() *ErrorResponse {
	return NewErrorResponse(
		http.StatusConflict,
		"CONFLICT",
		"This request has a conflict with the current state of the system.",
	)
}

// Gone is for 410 responses.
//
// This is appropriate for when the resource requested was once available but is
// no longer.
func Gone() *ErrorResponse {
	return NewErrorResponse(
		http.StatusGone,
		"GONE",
		"The requested resource is no longer available.",
	)
}

// PayloadTooLarge is for 413 responses.
//
// This is appropriate for when the client sends a request that is larger than
// the limits set by the server, such as when they try to upload a file that is
// too big.
func PayloadTooLarge() *ErrorResponse {
	return NewErrorResponse(
		http.StatusRequestEntityTooLarge,
		"PAYLOAD_TOO_LARGE",
		"The request was too large.",
	)
}

// UnsupportedMediaType is for 415 responses.
//
// This is appropriate for when the request has an unsupported media format.
func UnsupportedMediaType() *ErrorResponse {
	return NewErrorResponse(
		http.StatusUnsupportedMediaType,
		"UNSUPPORTED_MEDIA_TYPE",
		"The given media type is not supported.",
	)
}

// Teapot is for 418 responses.
//
// This is appropriate for when the server is a teapot rather than a coffee maker.
func Teapot() *ErrorResponse {
	return NewErrorResponse(
		http.StatusTeapot,
		"TEAPOT",
		"I am a teapot. ☕️",
	)
}

// UnprocessableEntity is for 422 responses.
//
// This is appropriate for when the request is valid but the server is unable to
// process the request instructions.
//
// The request should not be retried without modification.
func UnprocessableEntity() *ErrorResponse {
	return NewErrorResponse(
		http.StatusUnprocessableEntity,
		"UNPROCESSABLE_ENTITY",
		"The request could not be processed.",
	)
}

// TooEarly is for 425 responses.
//
// This is appropriate for when the server is concerned that the request may be
// replayed, resulting in a replay attack.
func TooEarly() *ErrorResponse {
	return NewErrorResponse(
		http.StatusTooEarly,
		"TOO_EARLY",
		"The server refused to process the request as it might be a replay.",
	)
}

// TooManyRequests is for 429 responses.
//
// This is appropriate for when the client has been rate limited by the server.
//
// It may be appropriate for the client to retry the request after some time has
// passed, it is encouraged to use this along with Retryable to communicate to
// the client when they are able to retry.
func TooManyRequests() *ErrorResponse {
	return NewErrorResponse(
		http.StatusTooManyRequests,
		"TOO_MANY_REQUESTS",
		"The client has sent too many requests to the server.",
	)
}

// LegalBlock is for 451 responses.
//
// This is appropriate for when the requested resource is unavailable for
// legal reasons, such as when the content is censored in a country.
func LegalBlock() *ErrorResponse {
	return NewErrorResponse(
		http.StatusUnavailableForLegalReasons,
		"UNAVAILABLE_FOR_LEGAL_REASONS",
		"The requested resource is unavailable for legal reason.",
	)
}

// InternalServerError is for 500 responses.
//
// This is appropriate for generic, unhandled server errors.
func InternalServerError() *ErrorResponse {
	return NewErrorResponse(
		http.StatusInternalServerError,
		"INTERNAL_SERVER_ERROR",
		"The server has either erred or is incapable of performing the request.",
	)
}

// NotImplemented is for 501 responses.
//
// This applies when a request is made for an HTTP method that the server
// understands but does not support.
func NotImplemented() *ErrorResponse {
	return NewErrorResponse(
		http.StatusNotImplemented,
		"NOT_IMPLEMENTED",
		"The server does not support the requirements for the request.",
	)
}

// BadGateway is for 502 responses.
//
// This is appropriate to use when your service is responsible for making
// requests to other services and one returns a bad (unexpected error or malformed)
// response.
func BadGateway() *ErrorResponse {
	return NewErrorResponse(
		http.StatusBadGateway,
		"BAD_GATEWAY",
		"The server received an invalid response from an upstream server.",
	)
}

// ServiceUnavailable is for 503 responses.
//
// This is appropriate for when a server is not ready to handle a request such
// as when it is down for maintenance or overloaded.
//
// Clients may retry 503's with exponential backoff.
func ServiceUnavailable() *ErrorResponse {
	return NewErrorResponse(
		http.StatusServiceUnavailable,
		"SERVICE_UNAVAILABLE",
		"The server is currently unavailable.\nPlease try again at a later time.",
	)
}

// GatewayTimeout is for 504 responses.
//
// This is appropriate to use when your service is responsible for making
// requests to other services and one times out.
func GatewayTimeout() *ErrorResponse {
	return NewErrorResponse(
		http.StatusGatewayTimeout,
		"GATEWAY_TIMEOUT",
		"The server timed out waiting for a response from an upstream service.",
	)
}

var errorFuncsByCode = map[int]func() *ErrorResponse{
	http.StatusBadRequest:                 BadRequest,
	http.StatusUnauthorized:               Unauthorized,
	http.StatusPaymentRequired:            PaymentRequired,
	http.StatusForbidden:                  Forbidden,
	http.StatusNotFound:                   NotFound,
	http.StatusMethodNotAllowed:           MethodNotAllowed,
	http.StatusConflict:                   Conflict,
	http.StatusGone:                       Gone,
	http.StatusRequestEntityTooLarge:      PayloadTooLarge,
	http.StatusUnsupportedMediaType:       UnsupportedMediaType,
	http.StatusTeapot:                     Teapot,
	http.StatusUnprocessableEntity:        UnprocessableEntity,
	http.StatusTooEarly:                   TooEarly,
	http.StatusTooManyRequests:            TooManyRequests,
	http.StatusUnavailableForLegalReasons: LegalBlock,
	http.StatusInternalServerError:        InternalServerError,
	http.StatusNotImplemented:             NotImplemented,
	http.StatusBadGateway:                 BadGateway,
	http.StatusServiceUnavailable:         ServiceUnavailable,
	http.StatusGatewayTimeout:             GatewayTimeout,
}

// ErrorForCode returns a new *ErrorResponse for the given HTTP status code if
// one is configured and falls back to returning InternalServerError() if the
// given code is not configured.
//
// This is intended to be used in cases where you have multiple potential error
// codes and want to return the appropriate error response.  If you are only
// returning a particular error, your code will likely be cleaner using the
// specific functions provided rather than using ErrorForCode.
//
// Supported codes are as follows:
//
//	// 4xx
//	http.StatusBadRequest:                 httpbp.BadRequest
//	http.StatusUnauthorized:               httpbp.Unauthorized
//	http.StatusPaymentRequired:            httpbp.PaymentRequired
//	http.StatusForbidden:                  httpbp.Forbidden
//	http.StatusNotFound:                   httpbp.NotFound
//	http.StatusMethodNotAllowed:           httpbp.MethodNotAllowed
//	http.StatusConflict:                   httpbp.Conflict
//	http.StatusGone:                       httpbp.Gone
//	http.StatusRequestEntityTooLarge:      httpbp.PayloadTooLarge
//	http.StatusUnsupportedMediaType:       httpbp.UnsupportedMediaType
//	http.StatusTeapot:                     httpbp.Teapot
//	http.StatusUnprocessableEntity:        httpbp.UnprocessableEntity
//	http.StatusTooEarly:                   httpbp.TooEarly
//	http.StatusTooManyRequests:            httpbp.TooManyRequests
//	http.StatusUnavailableForLegalReasons: httpbp.LegalBlock
//	// 5xx
//	http.StatusInternalServerError: httpbp.InternalServerError
//	http.StatusNotImplemented:      httpbp.NotImplemented
//	http.StatusBadGateway:          httpbp.BadGateway
//	http.StatusServiceUnavailable:  httpbp.ServiceUnavailable
//	http.StatusGatewayTimeout:      httpbp.GatewayTimeout
func ErrorForCode(code int) *ErrorResponse {
	if f, ok := errorFuncsByCode[code]; ok {
		return f()
	}
	return InternalServerError()
}

// ClientError defines the client side error constructed from an HTTP response.
//
// Please see ClientErrorFromResponse for more details.
type ClientError struct {
	Status     string
	StatusCode int
	RetryAfter time.Duration

	AdditionalInfo string
}

func (ce ClientError) Error() string {
	var sb strings.Builder
	sb.WriteString("httpbp.ClientError: ")
	if ce.Status == "" {
		sb.WriteString("nil response")
	} else {
		sb.WriteString("http status ")
		sb.WriteString(ce.Status)
	}
	if ce.AdditionalInfo != "" {
		sb.WriteString(": ")
		sb.WriteString(ce.AdditionalInfo)
	}
	return sb.String()
}

// RetryAfterDuration implements retrybp.RetryAfterError.
func (ce ClientError) RetryAfterDuration() time.Duration {
	return ce.RetryAfter
}

// Retryable implements retrybp.RetryableError.
//
// It returns true (1) on any of the following conditions and no decision (0)
// otherwise:
//
// - There was a valid Retry-After header in the response
//
// - The status code was one of:
//
//   * 425 (too early)
//   * 429 (too many requests)
//   * 503 (service unavailable)
func (ce ClientError) Retryable() int {
	if ce.StatusCode == 0 {
		// We didn't even get a response, not enough information to make a decision.
		return 0
	}

	if ce.RetryAfter > 0 {
		// If the server sent a valid Retry-After header,
		// we consider it as explicitly stating that it's retryable.
		return 1
	}

	switch ce.StatusCode {
	default:
		return 0

	case
		http.StatusTooEarly,
		http.StatusTooManyRequests,
		http.StatusServiceUnavailable:

		return 1
	}
}

// ClientErrorFromResponse creates ClientError from http response.
//
// It returns nil error when the response code are in range of [200, 400),
// or non-nil error otherwise (including response being nil).
// When the returned error is non-nil,
// it's guaranteed to be of type *ClientError.
//
// It does not read from resp.Body in any case,
// even if the returned error is non-nil.
// So it's always the caller's responsibility to read/close the body to ensure
// the HTTP connection can be reused with keep-alive.
//
// The caller can (optionally) choose to read the body and feed that back into
// ClientError.AdditionalInfo (it's always empty string when returned).
// See the example for more details regarding this.
func ClientErrorFromResponse(resp *http.Response) error {
	if resp == nil {
		return &ClientError{}
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return nil
	}
	ce := &ClientError{
		Status:     resp.Status,
		StatusCode: resp.StatusCode,
	}
	if retryAfter := strings.TrimSpace(resp.Header.Get(RetryAfterHeader)); retryAfter != "" {
		// Retry-After header could be either an absolute time or a relative time.
		t, err := http.ParseTime(retryAfter)
		if err == nil {
			ce.RetryAfter = time.Until(t)
		} else {
			// RFC says the relative time format of RetryAfter should be an integer,
			// but in reality floats could be used for better precision.
			seconds, err := strconv.ParseFloat(retryAfter, 64)
			if err == nil {
				ce.RetryAfter = time.Duration(seconds * float64(time.Second))
			}
		}
	}
	return ce
}

// DrainAndClose reads r fully then closes it.
//
// It's required for http response bodies by stdlib http clients to reuse
// keep-alive connections, so you should always defer it after checking error.
func DrainAndClose(r io.ReadCloser) error {
	var batch errorsbp.Batch
	_, err := io.Copy(io.Discard, r)
	batch.Add(err)
	batch.Add(r.Close())
	return batch.Compile()
}
