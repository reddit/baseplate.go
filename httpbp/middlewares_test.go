package httpbp_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/reddit/baseplate.go/ecinterface"
	"github.com/reddit/baseplate.go/httpbp"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/metadatabp"
	"github.com/reddit/baseplate.go/mqsend"
	"github.com/reddit/baseplate.go/tracing"
)

func TestWrap(t *testing.T) {
	t.Parallel()

	c := &counter{}
	if c.count != 0 {
		t.Fatal("Unexpected initial count.")
	}
	handler := httpbp.Wrap(
		"test",
		func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
			return nil
		},
		testMiddleware(c),
	)
	handler(context.Background(), nil, nil)
	if c.count != 1 {
		t.Fatalf("Unexpected count value %v", c.count)
	}
}

func TestInjectServerSpan(t *testing.T) {
	t.Parallel()

	defer func() {
		tracing.CloseTracer()
		tracing.InitGlobalTracer(tracing.Config{})
	}()
	mmq := mqsend.OpenMockMessageQueue(mqsend.MessageQueueConfig{
		MaxQueueSize:   100,
		MaxMessageSize: 1024,
	})
	logger, startFailing := tracing.TestWrapper(t)
	tracing.InitGlobalTracer(tracing.Config{
		SampleRate:               0,
		MaxRecordTimeout:         testTimeout,
		Logger:                   logger,
		TestOnlyMockMessageQueue: mmq,
	})
	startFailing()

	req := newRequest(t, "")

	cases := []struct {
		name           string
		truster        httpbp.HeaderTrustHandler
		err            error
		hasAnnotations bool
	}{
		{
			name:           "trust/no-err",
			truster:        httpbp.AlwaysTrustHeaders{},
			hasAnnotations: true,
		},
		{
			name:           "trust/err",
			truster:        httpbp.AlwaysTrustHeaders{},
			hasAnnotations: true,
			err:            errors.New("test"),
		},
		{
			name:           "trust/http-err/4xx",
			truster:        httpbp.AlwaysTrustHeaders{},
			hasAnnotations: true,
			err:            httpbp.JSONError(httpbp.BadRequest(), nil),
		},
		{
			name:           "trust/http-err/5xx",
			truster:        httpbp.AlwaysTrustHeaders{},
			hasAnnotations: true,
			err:            httpbp.JSONError(httpbp.InternalServerError(), nil),
		},
		{
			name:           "no-trust/no-err",
			truster:        httpbp.NeverTrustHeaders{},
			hasAnnotations: false,
		},
		{
			name:           "no-trust/err",
			truster:        httpbp.NeverTrustHeaders{},
			hasAnnotations: false,
			err:            errors.New("test"),
		},
	}
	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				handle := httpbp.Wrap(
					"test",
					newTestHandler(testHandlerPlan{err: c.err}),
					httpbp.InjectServerSpan(c.truster),
				)
				handle(req.Context(), httptest.NewRecorder(), req)

				ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
				defer cancel()
				msg, err := mmq.Receive(ctx)
				if !c.hasAnnotations && err == nil {
					t.Fatal("expected error, got nil")
				} else if c.hasAnnotations && err != nil {
					t.Fatal(err)
				}
				if !c.hasAnnotations {
					return
				}

				var trace tracing.ZipkinSpan
				err = json.Unmarshal(msg, &trace)
				if err != nil {
					t.Fatal(err)
				}
				if len(trace.BinaryAnnotations) == 0 {
					t.Fatal("no binary annotations")
				}
				t.Logf("%#v", trace.BinaryAnnotations)
				hasError := false
				for _, annotation := range trace.BinaryAnnotations {
					if annotation.Key == "error" {
						hasError = true
					}
				}
				expectedErr := c.err
				var httpErr httpbp.HTTPError
				if errors.As(c.err, &httpErr) {
					if httpErr.Response().Code < 500 {
						expectedErr = nil
					}
				}
				if expectedErr != nil && !hasError {
					t.Error("error binary annotation was not present.")
				} else if expectedErr == nil && hasError {
					t.Error("unexpected error binary annotation")
				}
			},
		)
	}
}

func TestInjectEdgeRequestContext(t *testing.T) {
	t.Parallel()

	const expectedHeader = "dummy-edge-context"

	impl := ecinterface.Mock()
	req := newRequest(t, expectedHeader)
	noHeader := newRequest(t, expectedHeader)
	noHeader.Header.Del(httpbp.EdgeContextHeader)

	cases := []struct {
		name           string
		truster        httpbp.HeaderTrustHandler
		request        *http.Request
		expectedHeader string
	}{
		{
			name:           "trust/header",
			truster:        httpbp.AlwaysTrustHeaders{},
			request:        req,
			expectedHeader: expectedHeader,
		},
		{
			name:    "trust/no-header",
			truster: httpbp.AlwaysTrustHeaders{},
			request: noHeader,
		},
		{
			name:    "no-trust",
			truster: httpbp.NeverTrustHeaders{},
			request: req,
		},
	}
	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				recorder := edgecontextRecorder{}
				handle := httpbp.Wrap(
					"test",
					newTestHandler(testHandlerPlan{}),
					httpbp.InjectEdgeRequestContext(httpbp.InjectEdgeRequestContextArgs{
						EdgeContextImpl: impl,
						TrustHandler:    c.truster,
						Logger:          log.TestWrapper(t),
					}),
					edgecontextRecorderMiddleware(impl, &recorder),
				)
				handle(c.request.Context(), httptest.NewRecorder(), c.request)

				if c.expectedHeader != recorder.header {
					t.Errorf("Expected edge-context header to be %q, got %q", c.expectedHeader, recorder.header)
				}
			},
		)
	}
}

func TestSupportedMethods(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name             string
		method           string
		supportedMethods []string
		errExpected      bool
	}{
		{
			name:             "head-supported-automatically-with-get/supported",
			method:           "HEAD",
			supportedMethods: []string{http.MethodGet},
			errExpected:      false,
		},
		{
			name:             "head-supported-automatically-with-get/not-supported",
			method:           "HEAD",
			supportedMethods: []string{http.MethodPost},
			errExpected:      true,
		},
		{
			name:             "post/supported",
			method:           http.MethodPost,
			supportedMethods: []string{http.MethodPost},
			errExpected:      false,
		},
		{
			name:             "post/not-supported",
			method:           http.MethodPost,
			supportedMethods: []string{http.MethodGet},
			errExpected:      true,
		},
		{
			name:             "multi/supported",
			method:           http.MethodGet,
			supportedMethods: []string{http.MethodPost, http.MethodGet},
			errExpected:      false,
		},
		{
			name:             "multi/not-supported",
			method:           http.MethodDelete,
			supportedMethods: []string{http.MethodPost, http.MethodGet},
			errExpected:      true,
		},
	}
	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				req := newRequest(t, "")
				req.Method = c.method
				handle := httpbp.Wrap(
					"test",
					newTestHandler(testHandlerPlan{}),
					httpbp.SupportedMethods(c.supportedMethods[0], c.supportedMethods[1:]...),
				)

				w := httptest.NewRecorder()
				err := handle(context.TODO(), w, req)
				if !c.errExpected && err != nil {
					t.Fatalf("unexpected error %v", err)
				} else if c.errExpected && err == nil {
					t.Fatal("expected an error, got nil")
				} else if !c.errExpected {
					return
				}

				var httpErr httpbp.HTTPError
				if errors.As(err, &httpErr) {
					if httpErr.Response().Code != http.StatusMethodNotAllowed {
						t.Errorf(
							"wronge response code, expected %d, got %d",
							http.StatusMethodNotAllowed,
							httpErr.Response().Code,
						)
					}
					if allow := w.Header().Get(httpbp.AllowHeader); allow != "" {
						hasGet := false
						hasHead := false
						for _, m := range c.supportedMethods {
							hasGet = hasGet || strings.Compare(m, http.MethodGet) == 0
							hasHead = hasHead || strings.Compare(m, http.MethodHead) == 0
						}
						if hasGet && !hasHead {
							c.supportedMethods = append(c.supportedMethods, http.MethodHead)
						}
						sort.Strings(c.supportedMethods)
						expected := strings.Join(c.supportedMethods, ",")
						if strings.Compare(expected, allow) != 0 {
							t.Errorf(
								"%q header did not match: expected %q, got %q",
								httpbp.AllowHeader,
								expected,
								allow,
							)
						}
					} else {
						t.Errorf("missing %q header", httpbp.AllowHeader)
					}
				} else {
					t.Fatalf("unexpected error type %v", err)
				}
			},
		)
	}
}

func TestInjectDebugHeaders(t *testing.T) {
	cases := []struct {
		name           string
		debugHeader    string
		expectedHeader string
		config         metadatabp.Config
	}{
		{
			name:           "debug header enabled",
			debugHeader:    "enabled",
			expectedHeader: "test_node-test_namespace-test_pod",
			config: metadatabp.Config{
				BaseK8sMetadata: map[metadatabp.BaseMetadata]string{
					metadatabp.BaseplateK8sNodeName:  "test_node",
					metadatabp.BaseplateK8sNodeIP:    "1.1.1.1",
					metadatabp.BaseplateK8sPodName:   "test_pod",
					metadatabp.BaseplateK8sPodIP:     "2.2.2.2",
					metadatabp.BaseplateK8sNamespace: "test_namespace",
				},
			},
		},
		{
			name:           "wrong value in debug header",
			debugHeader:    "wrong_value",
			expectedHeader: "",
			config: metadatabp.Config{
				BaseK8sMetadata: map[metadatabp.BaseMetadata]string{
					metadatabp.BaseplateK8sNodeName:  "test_node",
					metadatabp.BaseplateK8sNodeIP:    "1.1.1.1",
					metadatabp.BaseplateK8sPodName:   "test_pod",
					metadatabp.BaseplateK8sPodIP:     "2.2.2.2",
					metadatabp.BaseplateK8sNamespace: "test_namespace",
				},
			},
		},
		{
			name:           "no debug header",
			expectedHeader: "",
			config: metadatabp.Config{
				BaseK8sMetadata: map[metadatabp.BaseMetadata]string{
					metadatabp.BaseplateK8sNodeName:  "test_node",
					metadatabp.BaseplateK8sNodeIP:    "1.1.1.1",
					metadatabp.BaseplateK8sPodName:   "test_pod",
					metadatabp.BaseplateK8sPodIP:     "2.2.2.2",
					metadatabp.BaseplateK8sNamespace: "test_namespace",
				},
			},
		},
		{
			name:           "base value missing",
			expectedHeader: "",
			config: metadatabp.Config{
				BaseK8sMetadata: map[metadatabp.BaseMetadata]string{
					metadatabp.BaseplateK8sNodeIP:    "1.1.1.1",
					metadatabp.BaseplateK8sPodName:   "test_pod",
					metadatabp.BaseplateK8sPodIP:     "2.2.2.2",
					metadatabp.BaseplateK8sNamespace: "test_namespace",
				},
			},
		},
	}
	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				handle := httpbp.Wrap(
					"test",
					newTestHandler(testHandlerPlan{}),
					httpbp.InjectDebugHeaders(&c.config),
				)

				w := httptest.NewRecorder()
				req, err := http.NewRequest("get", "localhost:9090", strings.NewReader("test"))
				if err != nil {
					t.Fatal(err)
				}

				if c.debugHeader != "" {
					req.Header.Set(httpbp.RedditDebug, c.debugHeader)
				}
				handle(req.Context(), w, req)

				if c.expectedHeader != w.Header().Get(httpbp.RedditK8sMeta) {
					t.Errorf("Expected header to be %q, got %q", c.expectedHeader, w.Header().Get(httpbp.RedditK8sMeta))
				}
			},
		)
	}
}
