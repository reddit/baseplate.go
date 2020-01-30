package httpbp_test

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/go-kit/kit/endpoint"
	httpgk "github.com/go-kit/kit/transport/http"
	"github.com/reddit/baseplate.go/httpbp"
)

// ExampleResponse is an example response that implements the ErrorResponse
// interface.
type ExampleResponse struct {
	Message string `json:"message,omitempty"`

	err     httpbp.HTTPError
	cookies []*http.Cookie
	headers http.Header
	code    int
}

func (r *ExampleResponse) AddCookie(name, value string) {
	r.cookies = append(r.cookies, &http.Cookie{Name: name, Value: value})
}

func (r ExampleResponse) Cookies() []*http.Cookie {
	return r.cookies
}

func (r ExampleResponse) Headers() http.Header {
	return r.headers
}

func (r ExampleResponse) StatusCode() int {
	return r.code
}

// Err returns the internal HTTPError set on r.
func (r ExampleResponse) Err() error {
	return r.err
}

var (
	// Verify that both ExampleResponse and *ExampleResponse implement the
	// ResponseCookies interface.
	_ httpbp.ResponseCookies = ExampleResponse{}
	_ httpbp.ResponseCookies = (*ExampleResponse)(nil)
	// Verify that both ExampleResponse and *ExampleResponse implement the
	// go-kit http.Headerer interface.
	_ httpgk.Headerer = ExampleResponse{}
	_ httpgk.Headerer = (*ExampleResponse)(nil)
	// Verify that both ExampleResponse and *ExampleResponse implement the
	// go-kit http.StatusCoder interface.
	_ httpgk.StatusCoder = ExampleResponse{}
	_ httpgk.StatusCoder = (*ExampleResponse)(nil)
	// Verify that both ExampleResponse and *ExampleResponse implement the
	// ErrorResponse interface.
	_ httpbp.ErrorResponse = ExampleResponse{}
	_ httpbp.ErrorResponse = (*ExampleResponse)(nil)
)

// ExampleRequest is the request struct for our Example endpoint.
type ExampleRequest struct {
	// Error signals to the example endpoint whether it should return an error
	// or not.
	Error bool `json:"error"`
}

// DecodeExampleRequest decodes the request body into an ExampleRequest.
func DecodeExampleRequest(_ context.Context, r *http.Request) (interface{}, error) {
	var req ExampleRequest
	json.NewDecoder(r.Body).Decode(&req)
	return req, nil
}

// MakeExampleEndpoint builds a go-kit endpoint.Endpoint function that simply
// returns an ExampleResponse with an error set.
func MakeExampleEndpoint() endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		var resp ExampleResponse
		req := request.(ExampleRequest)
		if req.Error {
			// Return a response that returns a non-nil error when
			// httpbp.EncodeJSONResponse checks response.Err() which will signal it
			// to send an error response rather than a normal one.
			resp = ExampleResponse{
				Message: "you'll never see this",
				err: httpbp.HTTPError{
					// Code sets the status code to return, defaults to
					// http.InternalServerError (500).
					Code: http.StatusBadGateway,

					// Message sets a custom message for the error response
					// body, defaults to http.StatusText for the status code.
					Message: "Disruption downstream",

					// Cause holds the error that triggered an error response.
					// This is not communicated to the client but inspected by
					// your service.
					Cause: errors.New("database offline"),
				},
			}
		} else {
			// Return a non-error response.
			resp = ExampleResponse{Message: "hello world!"}
		}
		return resp, nil
	}
}

// This example demonstrates how to use HTTPError along with ErrorResponse to
// return errors to API clients.
//
// Example request and response:
//
// request: {"error": false}
// response:
//		code: 200
//		content-type: "application/json; charset=utf-8"
//		body: {"message": "hello world!"}
//
// request: {"error": true}
// response:
//		code: 502
//		content-type: "text/plain; charset=utf-8"
//		body: Disruption dowstream
func ExampleErrorResponse() {
	// Create server handler
	handler := http.NewServeMux()

	// Register our example endpoint
	handler.Handle("/example", httpgk.NewServer(
		MakeExampleEndpoint(),
		DecodeExampleRequest,
		httpbp.EncodeJSONResponse,
	))

	// Start the server
	log.Fatal(http.ListenAndServe(":8080", handler))
}
