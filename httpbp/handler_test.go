package httpbp_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/reddit/baseplate.go/httpbp"
)

func simplifyCookies(cookies []*http.Cookie) map[string][]string {
	cookieMap := make(map[string][]string)
	for _, cookie := range cookies {
		cookieMap[cookie.Name] = append(cookieMap[cookie.Name], cookie.Value)
	}
	for _, values := range cookieMap {
		sort.Strings(values)
	}
	return cookieMap
}

func TestHandler(t *testing.T) {
	t.Parallel()

	body := jsonResponseBody{X: 1}
	errResp := httpbp.BadGateway()
	headers := make(http.Header)
	headers.Add("foo", "bar")
	expectedCookie := &http.Cookie{Name: "fizz", Value: "buzz"}
	cookies := []*http.Cookie{expectedCookie}

	var successBody bytes.Buffer
	err := json.NewEncoder(&successBody).Encode(body)
	if err != nil {
		t.Fatal(err)
	}

	var jsonErrBody bytes.Buffer
	err = json.NewEncoder(&jsonErrBody).Encode(
		httpbp.ErrorResponseJSONWrapper{Error: errResp},
	)
	if err != nil {
		t.Fatal(err)
	}

	type expectation struct {
		code        int
		body        string
		headers     http.Header
		cookies     []*http.Cookie
		contentType string
	}
	cases := []struct {
		name         string
		plan         testHandlerPlan
		expectedBody string
		expected     expectation
	}{
		{
			name: "success",
			plan: testHandlerPlan{
				code:    http.StatusOK,
				body:    body,
				headers: headers,
				cookies: cookies,
			},
			expected: expectation{
				code:        http.StatusOK,
				body:        successBody.String(),
				headers:     headers,
				cookies:     cookies,
				contentType: httpbp.JSONContentType,
			},
		},
		{
			name: "unhandled-error",
			plan: testHandlerPlan{
				code:    http.StatusOK,
				body:    body,
				headers: headers,
				cookies: cookies,
				err:     errors.New("test"),
			},
			expected: expectation{
				code:        http.StatusInternalServerError,
				body:        http.StatusText(http.StatusInternalServerError) + "\n",
				headers:     headers,
				cookies:     cookies,
				contentType: httpbp.PlainTextContentType,
			},
		},
		{
			name: "HTTPError",
			plan: testHandlerPlan{
				code:    http.StatusOK,
				body:    body,
				headers: headers,
				cookies: cookies,
				err:     httpbp.JSONError(errResp, errors.New("test")),
			},
			expected: expectation{
				code:        http.StatusBadGateway,
				body:        jsonErrBody.String(),
				headers:     headers,
				cookies:     cookies,
				contentType: httpbp.JSONContentType,
			},
		},
	}

	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				t.Parallel()
				handler := httpbp.NewHandler("test", newTestHandler(c.plan))
				request := httptest.NewRequest(
					"get",
					"localhost:9090",
					strings.NewReader(""),
				)
				resp := httptest.NewRecorder()
				handler.ServeHTTP(resp, request)

				if resp.Body.String() != c.expected.body {
					t.Errorf(
						"body does not match, expected %q, got %q",
						c.expected.body,
						resp.Body,
					)
				}
				if resp.Code != c.expected.code {
					t.Errorf("wrong status code %d", resp.Code)
				}
				for k, respValues := range resp.Header() {
					if k == "Set-Cookie" || k == httpbp.ContentTypeHeader || k == "X-Content-Type-Options" {
						continue
					}
					values := c.expected.headers.Values(k)
					if !reflect.DeepEqual(respValues, values) {
						t.Errorf(
							"headers mismatch, expected %q:%#v, got %q:%#v",
							k,
							values,
							k,
							respValues,
						)
					}
				}
				if resp.Header().Get(httpbp.ContentTypeHeader) != c.expected.contentType {
					t.Errorf(
						"content-type mismatch, expected %q, got %q",
						resp.Header().Get(httpbp.ContentTypeHeader),
						c.expected.contentType,
					)
				}
				respCookies := simplifyCookies(resp.Result().Cookies())
				expectedCookies := simplifyCookies(c.expected.cookies)
				if !reflect.DeepEqual(respCookies, expectedCookies) {
					t.Errorf("cookies mismatch, expected %#v, got %#v", expectedCookies, respCookies)
				}
			},
		)
	}
}
