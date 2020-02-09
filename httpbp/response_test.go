package httpbp_test

import (
	"context"
	"errors"
	"html/template"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpgk "github.com/go-kit/kit/transport/http"
	"github.com/reddit/baseplate.go/httpbp"
)

func TestHTTPError(t *testing.T) {
	t.Parallel()

	t.Run(
		"default values",
		func(t *testing.T) {
			t.Parallel()

			defaultCode := http.StatusInternalServerError
			err := httpbp.HTTPError{}
			if err.StatusCode() != defaultCode {
				t.Errorf("Got unexpected status code %d", err.StatusCode())
			}
			if err.Unwrap() != nil {
				t.Errorf("Expected Unwrap to return nil: %v", err.Unwrap())
			}
			if err.ResponseMessage() != http.StatusText(defaultCode) {
				t.Errorf("Got unexpected response message: %v", err.ResponseMessage())
			}
		},
	)

	t.Run(
		"custom values",
		func(t *testing.T) {
			t.Parallel()

			cause := errors.New("test-error")
			message := "test-message"
			err := httpbp.HTTPError{
				Code:    http.StatusForbidden,
				Message: message,
				Cause:   cause,
			}
			if err.StatusCode() != http.StatusForbidden {
				t.Errorf("Got unexpected status code %d", err.StatusCode())
			}
			if err.Unwrap() != cause {
				t.Errorf("Expected Unwrap to return %v, got %v", cause, err.Unwrap())
			}
			if err.ResponseMessage() != message {
				t.Errorf("Got unexpected response message: %v", err.ResponseMessage())
			}
		},
	)

	t.Run(
		"code only",
		func(t *testing.T) {
			t.Parallel()

			err := httpbp.HTTPError{
				Code: http.StatusForbidden,
			}
			if err.StatusCode() != http.StatusForbidden {
				t.Errorf("Got unexpected status code %d", err.StatusCode())
			}
			if err.Unwrap() != nil {
				t.Errorf("Expected Unwrap to return nil: %v", err.Unwrap())
			}
			if err.ResponseMessage() != http.StatusText(http.StatusForbidden) {
				t.Errorf("Got unexpected response message: %v", err.ResponseMessage())
			}
		},
	)

	t.Run(
		"as",
		func(t *testing.T) {
			t.Parallel()

			code := http.StatusForbidden
			errs := []error{
				httpbp.HTTPError{Code: code},
				&httpbp.HTTPError{Code: code},
			}
			for _, err := range errs {
				var he httpbp.HTTPError
				if !errors.As(err, &he) {
					log.Fatalf("errors.As failed")
				}
				if he.StatusCode() != code {
					t.Errorf("Got unexpected status code %d", he.StatusCode())
				}
			}
		},
	)
}

type testResponse struct {
	httpbp.BaseResponse

	Message string `json:"message,omitempty"`
}

func (r *testResponse) AddCookie(name, value string) {
	r.SetCookie(&http.Cookie{Name: name, Value: value})
}

var (
	_ httpgk.Headerer        = testResponse{}
	_ httpgk.Headerer        = (*testResponse)(nil)
	_ httpgk.StatusCoder     = testResponse{}
	_ httpgk.StatusCoder     = (*testResponse)(nil)
	_ httpbp.ErrorResponse   = testResponse{}
	_ httpbp.ErrorResponse   = (*testResponse)(nil)
	_ httpbp.ResponseCookies = testResponse{}
	_ httpbp.ResponseCookies = (*testResponse)(nil)
)

func TestEncodeJSONResponse(t *testing.T) {
	t.Parallel()

	t.Run(
		"empty",
		func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			resp := &testResponse{}
			httpbp.EncodeJSONResponse(context.Background(), w, resp)
			result := w.Result()

			if result.StatusCode != http.StatusOK {
				t.Errorf("unexpected status code: %d", result.StatusCode)
			}

			if len(result.Header) != 1 {
				t.Errorf("too many headers: %v.", w.Header())
			}

			if result.Header.Get(httpbp.ContentTypeHeader) != httpbp.JSONContentType {
				t.Errorf("wrong content-type: %v", result.Header.Get(httpbp.ContentTypeHeader))
			}

			body := make([]byte, 1024)
			if _, err := result.Body.Read(body); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			str := strings.Trim(string(body), "\x00\n")
			if strings.Compare(str, "{}") != 0 {
				t.Errorf("unexpected body: %q", str)
			}

			if len(result.Cookies()) != 0 {
				t.Errorf("too many cookies: %v", result.Cookies())
			}
		},
	)

	t.Run(
		"populated",
		func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			resp := &testResponse{
				BaseResponse: httpbp.NewBaseResponse(),
				Message:      "test-message",
			}
			resp.SetCode(http.StatusAccepted)
			resp.AddCookie("test-cookie", "foo")
			resp.Headers().Add("test-header", "bar")
			httpbp.EncodeJSONResponse(context.Background(), w, resp)
			result := w.Result()

			if result.StatusCode != http.StatusAccepted {
				t.Errorf("unexpected status code: %d", result.StatusCode)
			}

			if result.Header.Get(httpbp.ContentTypeHeader) != httpbp.JSONContentType {
				t.Errorf("wrong content-type: %v", result.Header.Get(httpbp.ContentTypeHeader))
			}
			if result.Header.Get("test-header") != "bar" {
				t.Errorf("wrong test-header: %v", result.Header.Get("test-header"))
			}

			if len(result.Cookies()) != 1 {
				t.Errorf("wrong number of cookies: %v", result.Cookies())
			}

			body := make([]byte, 1024)
			if _, err := result.Body.Read(body); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			str := strings.Trim(string(body), "\x00\n")
			if str != "{\"message\":\"test-message\"}" {
				t.Errorf("unexpected body: %q", str)
			}
		},
	)

	t.Run(
		"error",
		func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			resp := &testResponse{
				BaseResponse: httpbp.NewBaseResponse(),
				Message:      "test-message",
			}
			resp.SetCode(http.StatusAccepted)
			resp.SetError(httpbp.HTTPError{
				Code: http.StatusForbidden,
			})
			resp.AddCookie("test-cookie", "foo")
			resp.Headers().Add("test-header", "bar")
			httpbp.EncodeJSONResponse(context.Background(), w, resp)
			result := w.Result()

			if result.StatusCode != resp.Err().(httpbp.HTTPError).StatusCode() {
				t.Errorf("unexpected status code: %d", result.StatusCode)
			}

			if result.Header.Get("test-header") != "bar" {
				t.Errorf("wrong test-header: %v", result.Header.Get("test-header"))
			}

			if len(result.Cookies()) != 1 {
				t.Errorf("wrong number of cookies: %v", result.Cookies())
			}

			body := make([]byte, 1024)
			if _, err := result.Body.Read(body); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			str := strings.Trim(string(body), "\x00\n")
			if str != "Forbidden" {
				t.Errorf("unexpected body: %q", str)
			}
		},
	)

	t.Run(
		"clear cookies",
		func(t *testing.T) {
			t.Parallel()

			resp := &testResponse{
				BaseResponse: httpbp.NewBaseResponse(),
			}
			resp.AddCookie("test", "cookie")
			if len(resp.Cookies()) != 1 {
				t.Fatalf("Expected 1 cookie. %#v", resp.Cookies())
			}

			resp.ClearCookies()
			if len(resp.Cookies()) != 0 {
				t.Fatalf("Expected 0 cookies. %#v", resp.Cookies())
			}
		},
	)
}

func TestEncodeTemplatedResponse(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	resp := &testResponse{
		BaseResponse: httpbp.NewBaseResponse(),
		Message:      "test-message",
	}
	resp.SetCode(http.StatusAccepted)
	resp.AddCookie("test-cookie", "foo")
	resp.Headers().Add("test-header", "bar")
	temp, err := template.New("test").Parse(`{{.Message}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	httpbp.EncodeTemplatedResponse(context.Background(), w, resp, temp)
	result := w.Result()

	if result.StatusCode != http.StatusAccepted {
		t.Errorf("unexpected status code: %d", result.StatusCode)
	}

	if result.Header.Get(httpbp.ContentTypeHeader) != httpbp.HTMLContentType {
		t.Errorf("wrong content-type: %v", result.Header.Get(httpbp.ContentTypeHeader))
	}
	if result.Header.Get("test-header") != "bar" {
		t.Errorf("wrong test-header: %v", result.Header.Get("test-header"))
	}

	if len(result.Cookies()) != 1 {
		t.Errorf("wrong number of cookies: %v", result.Cookies())
	}

	body := make([]byte, 1024)
	if _, err := result.Body.Read(body); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	str := strings.Trim(string(body), "\x00\n")
	if str != "test-message" {
		t.Errorf("unexpected body: %q", str)
	}
}
