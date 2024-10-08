package httpbp_test

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/reddit/baseplate.go"
	"github.com/reddit/baseplate.go/ecinterface"
	"github.com/reddit/baseplate.go/httpbp"
	"github.com/reddit/baseplate.go/log"
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

func TestMiddlewareResponseWrapping(t *testing.T) {
	store := newSecretsStore(t)
	defer store.Close()

	bp := baseplate.NewTestBaseplate(baseplate.NewTestBaseplateArgs{
		Config:          baseplate.Config{Addr: ":8080"},
		Store:           store,
		EdgeContextImpl: ecinterface.Mock(),
	})

	args := httpbp.ServerArgs{
		Baseplate: bp,
		Middlewares: []httpbp.Middleware{
			func(name string, next httpbp.HandlerFunc) httpbp.HandlerFunc {
				return func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
					if flusher, isFlusher := w.(http.Flusher); isFlusher {
						flusher.Flush()
					}

					next(ctx, w, r)
					return nil
				}
			},
			func(name string, next httpbp.HandlerFunc) httpbp.HandlerFunc {
				return func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
					if hijacker, isHijacker := w.(http.Hijacker); isHijacker {
						hijacker.Hijack()
					}

					next(ctx, w, r)
					return nil
				}
			},
			func(name string, next httpbp.HandlerFunc) httpbp.HandlerFunc {
				return func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
					if pusher, isPusher := w.(http.Pusher); isPusher {
						pusher.Push("target", &http.PushOptions{})
					}

					next(ctx, w, r)
					return nil
				}
			},
		},
		Endpoints: map[httpbp.Pattern]httpbp.Endpoint{
			"/test": {
				Name:    "test",
				Methods: []string{http.MethodGet},
				Handle: func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
					w.Write([]byte("endpoint"))
					return nil
				},
			},
		},
	}

	// register our middleware to the EndpointRegistry
	args, err := args.SetupEndpoints()

	if err != nil {
		t.Fatal(err)
	}

	t.Run("non-flushable-non-hijackable", func(tt *testing.T) {
		type baseResponseWriter struct {
			http.ResponseWriter
		}

		r := httptest.NewRequest(http.MethodGet, "/test", nil)
		inner := httptest.NewRecorder()
		w := baseResponseWriter{inner}
		args.EndpointRegistry.ServeHTTP(w, r)

		if inner.Flushed {
			tt.Error("expected response to not be flushed")
		}
	})

	// Test the a flushable response
	t.Run("flushable", func(tt *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		args.EndpointRegistry.ServeHTTP(w, r)

		if !w.Flushed {
			tt.Error("expected http response to be flushed")
		}
	})

	t.Run("hijackable", func(tt *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := &hijackableResponseRecorder{httptest.NewRecorder(), false}
		args.EndpointRegistry.ServeHTTP(w, r)

		if !w.Hijacked {
			tt.Error("expected http response to be hijacked")
		}
	})

	t.Run("pushable", func(tt *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := &pushableResponseRecorder{httptest.NewRecorder(), false}
		args.EndpointRegistry.ServeHTTP(w, r)

		if !w.Pushed {
			tt.Error("expected http response to be pushed")
		}
	})

	t.Run("hijackable-flushable", func(tt *testing.T) {
		type hijackableFlushableRecorder struct {
			hijackableResponseRecorder
			http.Flusher
		}

		r := httptest.NewRequest(http.MethodGet, "/test", nil)
		inner := httptest.NewRecorder()
		w := &hijackableFlushableRecorder{
			hijackableResponseRecorder{inner, false},
			inner,
		}
		args.EndpointRegistry.ServeHTTP(w, r)

		if !w.Hijacked {
			tt.Error("expected http response to be hijacked")
		}

		if !inner.Flushed {
			tt.Error("expected http response to be flushed")
		}
	})
}

type hijackableResponseRecorder struct {
	http.ResponseWriter
	Hijacked bool
}

func (h *hijackableResponseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h.Hijacked = true
	return nil, nil, nil
}

type pushableResponseRecorder struct {
	http.ResponseWriter
	Pushed bool
}

func (p *pushableResponseRecorder) Push(target string, opts *http.PushOptions) error {
	p.Pushed = true
	return nil
}
