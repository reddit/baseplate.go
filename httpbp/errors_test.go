package httpbp_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/reddit/baseplate.go/httpbp"
	"github.com/reddit/baseplate.go/retrybp"
)

func TestErrorResponse(t *testing.T) {
	t.Parallel()

	cause := errors.New("test")
	tmpl, err := httpbp.RegisterDefaultErrorTemplate(template.New("test"))
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		resp *httpbp.ErrorResponse
		code int
	}{
		// 4xx
		{
			resp: httpbp.BadRequest(),
			code: http.StatusBadRequest,
		},
		{
			resp: httpbp.Unauthorized(),
			code: http.StatusUnauthorized,
		},
		{
			resp: httpbp.PaymentRequired(),
			code: http.StatusPaymentRequired,
		},
		{
			resp: httpbp.Forbidden(),
			code: http.StatusForbidden,
		},
		{
			resp: httpbp.NotFound(),
			code: http.StatusNotFound,
		},
		{
			resp: httpbp.MethodNotAllowed(),
			code: http.StatusMethodNotAllowed,
		},
		{
			resp: httpbp.Conflict(),
			code: http.StatusConflict,
		},
		{
			resp: httpbp.Gone(),
			code: http.StatusGone,
		},
		{
			resp: httpbp.PayloadTooLarge(),
			code: http.StatusRequestEntityTooLarge,
		},
		{
			resp: httpbp.UnsupportedMediaType(),
			code: http.StatusUnsupportedMediaType,
		},
		{
			resp: httpbp.Teapot(),
			code: http.StatusTeapot,
		},
		{
			resp: httpbp.UnprocessableEntity(),
			code: http.StatusUnprocessableEntity,
		},
		{
			resp: httpbp.TooEarly(),
			code: http.StatusTooEarly,
		},
		{
			resp: httpbp.TooManyRequests(),
			code: http.StatusTooManyRequests,
		},
		{
			resp: httpbp.LegalBlock(),
			code: http.StatusUnavailableForLegalReasons,
		},
		// 5xx
		{
			resp: httpbp.InternalServerError(),
			code: http.StatusInternalServerError,
		},
		{
			resp: httpbp.NotImplemented(),
			code: http.StatusNotImplemented,
		},
		{
			resp: httpbp.BadGateway(),
			code: http.StatusBadGateway,
		},
		{
			resp: httpbp.ServiceUnavailable(),
			code: http.StatusServiceUnavailable,
		},
		{
			resp: httpbp.GatewayTimeout(),
			code: http.StatusGatewayTimeout,
		},
	}

	for _, _c := range cases {
		c := _c
		t.Run(
			c.resp.Reason,
			func(t *testing.T) {
				t.Run(
					"ErrorForCode",
					func(t *testing.T) {
						resp := httpbp.ErrorForCode(c.code)

						if len(resp.Details) != 0 {
							t.Error("details should be empty by default")
						}

						err := httpbp.JSONError(resp, cause)

						code := err.Response().Code
						if code != c.code {
							t.Errorf("code mismatch, expected %d, got %d", c.code, code)
						}

						reasonEqual := strings.Compare(resp.Reason, c.resp.Reason) == 0
						explanationEqual := strings.Compare(resp.Explanation, c.resp.Explanation) == 0

						if !reasonEqual || !explanationEqual {
							t.Errorf(
								"ErrorForCode did not return the expected error response, expected %#v, got %#v",
								c.resp,
								resp,
							)
						}
					},
				)

				t.Run(
					"HTTPError",
					func(t *testing.T) {
						err := httpbp.JSONError(c.resp, cause)

						code := err.Response().Code
						if code != c.code {
							t.Errorf("code mismatch, expected %d, got %d", c.code, code)
						}

						// This is testing HTTPError.Unwrap under the hood.
						if !errors.Is(err, cause) {
							t.Error("Unwrap should result in errors.Is(HTTPErr, cause) to return true")
						}
					},
				)

				t.Run(
					"JSON",
					func(t *testing.T) {
						err := httpbp.JSONError(c.resp, cause)

						cw := err.ContentWriter()
						if cw.ContentType() != httpbp.JSONContentType {
							t.Fatal("wrong content writer")
						}

						w := httptest.NewRecorder()
						httpbp.WriteResponse(w, cw, err.Response())

						resp := httpbp.ErrorResponseJSONWrapper{}
						body := w.Body.String()
						if err := json.NewDecoder(strings.NewReader(body)).Decode(&resp); err != nil {
							t.Fatal(err)
						}

						if resp.Success {
							t.Error("success should be false")
						}
						if resp.Error.Reason != c.resp.Reason {
							t.Error(w.Body.String())
							t.Errorf(
								"reason mismatch, expected %q, got %q",
								c.resp.Reason,
								resp.Error.Reason,
							)
						}
						if resp.Error.Explanation != c.resp.Explanation {
							t.Errorf(
								"explanation mismatch, expected %q, got %q",
								c.resp.Explanation,
								resp.Error.Explanation,
							)
						}
						if resp.Error.Details == nil {
							resp.Error.Details = make(map[string]string)
						}
						if !reflect.DeepEqual(resp.Error.Details, c.resp.Details) {
							t.Errorf(
								"details mismatch, expected %#v, got %#v",
								c.resp.Details,
								resp.Error.Details,
							)
						}
					},
				)

				t.Run(
					"HTML",
					func(t *testing.T) {
						err := httpbp.HTMLError(c.resp, cause, tmpl)

						cw := err.ContentWriter()
						if cw.ContentType() != httpbp.HTMLContentType {
							t.Fatal("wrong content writer")
						}

						w := httptest.NewRecorder()
						httpbp.WriteResponse(w, cw, err.Response())

						body := w.Body.String()
						if i := strings.Index(body, c.resp.Reason); i == -1 {
							t.Errorf("reason missing from response")
						}
						if i := strings.Index(body, c.resp.Explanation); i == -1 {
							t.Errorf("explanation missing from response")
						}
					},
				)

				t.Run(
					"Raw",
					func(t *testing.T) {
						err := httpbp.RawError(c.resp, cause, httpbp.PlainTextContentType)

						cw := err.ContentWriter()
						if cw.ContentType() != httpbp.PlainTextContentType {
							t.Fatal("wrong content writer")
						}

						w := httptest.NewRecorder()
						httpbp.WriteResponse(w, cw, err.Response())

						body := w.Body.String()
						if body != c.resp.String() {
							t.Errorf("body mismatch, expected %q, got %q", c.resp, body)
						}
					},
				)
			},
		)
	}
}

func TestWithDetails(t *testing.T) {
	key := "foo"
	value := "bar"
	tmpl, err := httpbp.RegisterDefaultErrorTemplate(template.New("test"))
	if err != nil {
		t.Fatal(err)
	}

	resp := httpbp.BadRequest().WithDetails(map[string]string{
		key: value,
	})
	if v, ok := resp.Details[key]; !ok || v != value {
		t.Fatal("key not set in details properly")
	}

	t.Run(
		"JSON",
		func(t *testing.T) {
			err := httpbp.JSONError(resp, nil)
			w := httptest.NewRecorder()
			httpbp.WriteResponse(w, err.ContentWriter(), err.Response())

			got := httpbp.ErrorResponseJSONWrapper{}
			body := w.Body.String()
			if err := json.NewDecoder(strings.NewReader(body)).Decode(&got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.Error.Details, resp.Details) {
				t.Errorf(
					"details mismatch, expected %#v, got %#v",
					resp.Details,
					got.Error.Details,
				)
			}
		},
	)

	t.Run(
		"HTML",
		func(t *testing.T) {
			err := httpbp.HTMLError(resp, nil, tmpl)
			w := httptest.NewRecorder()
			httpbp.WriteResponse(w, err.ContentWriter(), err.Response())

			body := w.Body.String()
			if i := strings.Index(body, key); i == -1 {
				t.Errorf("details.key missing from response")
			}
			if i := strings.Index(body, value); i == -1 {
				t.Errorf("details.value missing from response")
			}
		},
	)
}

func TestWithRawResponse(t *testing.T) {
	t.Parallel()

	raw := "test"
	resp := httpbp.InternalServerError().WithRawResponse(raw)
	if resp.String() != raw {
		t.Errorf("raw response mismatch, expected %q, got %q", raw, resp.String())
	}

	err := httpbp.RawError(resp, nil, httpbp.PlainTextContentType)
	w := httptest.NewRecorder()
	httpbp.WriteResponse(w, err.ContentWriter(), err.Response())

	body := w.Body.String()
	if body != raw {
		t.Errorf("body mismatch, expected %q, got %q", raw, body)
	}
}

func TestWithTemplateName(t *testing.T) {
	t.Parallel()

	name := "test"
	text := "foo"
	resp := httpbp.InternalServerError().WithTemplateName(name)
	if resp.TemplateName() != name {
		t.Errorf("template name mismatch, expected %q, got %q", name, resp.TemplateName())
	}

	tmpl, err := template.New(name).Parse(text)
	if err != nil {
		t.Fatalf("Template parsing failed: %v", err)
	}
	tmpl, err = httpbp.RegisterDefaultErrorTemplate(tmpl)
	if err != nil {
		t.Fatal(err)
	}

	htmlErr := httpbp.HTMLError(resp, nil, tmpl)
	w := httptest.NewRecorder()
	httpbp.WriteResponse(w, htmlErr.ContentWriter(), htmlErr.Response())
	body := w.Body.String()
	if body != text {
		t.Errorf("body mismatch, expected %q, got %q", text, body)
	}
}

func TestRegisterCustomDefaultErrorTemplate(t *testing.T) {
	t.Parallel()

	text := "foo"
	resp := httpbp.InternalServerError()
	if resp.TemplateName() != httpbp.DefaultErrorTemplateName {
		t.Errorf(
			"template name mismatch, expected %q, got %q",
			httpbp.DefaultErrorTemplateName,
			resp.TemplateName(),
		)
	}

	tmpl, err := httpbp.RegisterCustomDefaultErrorTemplate(template.New("test"), text)
	if err != nil {
		t.Fatal(err)
	}

	htmlErr := httpbp.HTMLError(resp, nil, tmpl)
	w := httptest.NewRecorder()
	httpbp.WriteResponse(w, htmlErr.ContentWriter(), htmlErr.Response())
	body := w.Body.String()
	if body != text {
		t.Errorf("body mismatch, expected %q, got %q", text, body)
	}
}

func TestRetryable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "Millisecond",
			duration: time.Millisecond,
			expected: "0.001",
		},
		{
			name:     "Hundred-Milliseconds",
			duration: time.Millisecond * 100,
			expected: "0.1",
		},
		{
			name:     "Second",
			duration: time.Second,
			expected: "1",
		},
		{
			name:     "Minute",
			duration: time.Minute,
			expected: "60",
		},
		{
			name:     "Hour",
			duration: time.Hour,
			expected: "3600",
		},
	}

	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {

				w := httptest.NewRecorder()
				httpbp.TooManyRequests().Retryable(w, c.duration)

				duration := w.Header().Get(httpbp.RetryAfterHeader)
				if duration != c.expected {
					t.Errorf("headers do not match, expected %q, got %q", c.expected, duration)
				}
			},
		)
	}
}

func TestClientError(t *testing.T) {
	for _, c := range []struct {
		label              string
		handler            http.HandlerFunc
		expectError        bool
		expectedCode       int
		expectedRetryAfter time.Duration
	}{
		{
			label: "normal",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				fmt.Fprintf(w, "hello")
			},
		},
		{
			label: "normal-with-retry-after",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				httpbp.ServiceUnavailable().Retryable(w, time.Minute)
				fmt.Fprintf(w, "hello")
			},
		},
		{
			label: "502",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(502)
				fmt.Fprintf(w, "hello")
			},
			expectError:  true,
			expectedCode: 502,
		},
		{
			label: "retry-after",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				httpbp.ServiceUnavailable().Retryable(w, time.Minute)
				w.WriteHeader(502)
				fmt.Fprintf(w, "hello")
			},
			expectError:        true,
			expectedCode:       502,
			expectedRetryAfter: time.Minute,
		},
		{
			label: "sub-second-retry-after",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				httpbp.ServiceUnavailable().Retryable(w, time.Second/2)
				w.WriteHeader(502)
				fmt.Fprintf(w, "hello")
			},
			expectError:        true,
			expectedCode:       502,
			expectedRetryAfter: time.Second / 2,
		},
	} {
		t.Run(c.label, func(t *testing.T) {
			ts := httptest.NewServer(c.handler)
			defer ts.Close()

			resp, err := http.Get(ts.URL)
			if err != nil {
				t.Fatal(err)
			}
			err = httpbp.ClientErrorFromResponse(resp)

			if err == nil {
				if c.expectError {
					t.Error("Expected error, got nil")
				}
				return
			}

			if !c.expectError {
				t.Fatalf("Did not expect error, got %v", err)
			}
			var ce *httpbp.ClientError
			if !errors.As(err, &ce) {
				t.Fatalf("Expected err to be of type *httpbp.ClientError, got %T: %v", err, err)
			}

			if ce.StatusCode != c.expectedCode {
				t.Errorf("Expected status code %d, got %d", c.expectedCode, ce.StatusCode)
			}

			var rae retrybp.RetryAfterError
			if !errors.As(err, &rae) {
				t.Fatalf("Expected err to implement retrybp.RetryAfterError, got %T: %v", err, err)
			}
			actual := rae.RetryAfterDuration()
			if actual != c.expectedRetryAfter {
				t.Errorf("Expected RetryAfter %v, got %v", c.expectedRetryAfter, actual)
			}
		})
	}
}

var _ retrybp.RetryableError = httpbp.ClientError{}
