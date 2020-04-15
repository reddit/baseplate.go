package httpbp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/reddit/baseplate.go/edgecontext"
	"github.com/reddit/baseplate.go/httpbp"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/mqsend"
	"github.com/reddit/baseplate.go/tracing"
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
	errBody := jsonResponseBody{X: 2}
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
	err = json.NewEncoder(&jsonErrBody).Encode(errBody)
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
				err: httpbp.NewJSONError(
					http.StatusBadGateway,
					errBody,
					errors.New("test"),
				),
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
				handler := httpbp.NewHandler(newTestHandler(c.plan), "test")
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

func TestBaseplateHandlerFactory(t *testing.T) {
	defer func() {
		tracing.CloseTracer()
		tracing.InitGlobalTracer(tracing.TracerConfig{})
	}()

	const expectedID = "t2_example"

	mmq := mqsend.OpenMockMessageQueue(mqsend.MessageQueueConfig{
		MaxQueueSize:   100,
		MaxMessageSize: 1024,
	})
	logger, startFailing := tracing.TestWrapper(t)
	tracing.InitGlobalTracer(tracing.TracerConfig{
		SampleRate:               1,
		MaxRecordTimeout:         testTimeout,
		Logger:                   logger,
		TestOnlyMockMessageQueue: mmq,
	})
	startFailing()

	store, dir := newSecretsStore(t)
	defer os.RemoveAll(dir)
	defer store.Close()

	c := counter{}
	ecRecorder := edgecontextRecorder{}
	factory := httpbp.BaseplateHandlerFactory{
		Args: httpbp.DefaultMiddlewareArgs{
			TrustHandler:    httpbp.AlwaysTrustHeaders{},
			EdgeContextImpl: edgecontext.Init(edgecontext.Config{Store: store}),
			Logger:          log.TestWrapper(t),
		},
		Middlewares: []httpbp.Middleware{
			edgecontextRecorderMiddleware(&ecRecorder),
			testMiddleware(&c),
		},
	}

	handle := newTestHandler(testHandlerPlan{
		body: jsonResponseBody{X: 1},
	})
	// include an additional counter middleware here to test that both middlewares
	// are applied
	handler := factory.NewHandler("test", handle, testMiddleware(&c))
	handler.ServeHTTP(httptest.NewRecorder(), newRequest(t))

	// c.count should be 2 since we applied the "counting" middleware twice
	if c.count != 2 {
		t.Errorf("wrong counter %d", c.count)
	}

	// verify that the Span middleware was applied
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	msg, err := mmq.Receive(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var trace tracing.ZipkinSpan
	err = json.Unmarshal(msg, &trace)
	if err != nil {
		t.Fatal(err)
	}
	if len(trace.BinaryAnnotations) == 0 {
		t.Fatal("no binary annotations")
	}

	// verify that the EdgeContext middleware was applied
	if ecRecorder.EdgeContext == nil {
		t.Fatal("edge request context not set")
	}
	userID, ok := ecRecorder.EdgeContext.User().ID()
	if !ok {
		t.Error("user should be logged in")
	}
	if userID != expectedID {
		t.Errorf("user ID mismatch, expected %q, got %q", expectedID, userID)
	}
}
