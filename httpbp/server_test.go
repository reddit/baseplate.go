package httpbp_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/reddit/baseplate.go"
	"github.com/reddit/baseplate.go/ecinterface"
	"github.com/reddit/baseplate.go/httpbp"
	"github.com/reddit/baseplate.go/log"
)

func TestEndpoint(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		endpoint httpbp.Endpoint
		expected bool
	}{
		{
			name: "default",
			endpoint: httpbp.Endpoint{
				Name:    "test",
				Methods: []string{http.MethodGet},
				Handle: func(context.Context, http.ResponseWriter, *http.Request) error {
					return nil
				},
				Middlewares: []httpbp.Middleware{
					func(_ string, next httpbp.HandlerFunc) httpbp.HandlerFunc {
						return next
					},
				},
			},
			expected: false,
		},
		{
			name: "no-middlewares",
			endpoint: httpbp.Endpoint{
				Name:    "test",
				Methods: []string{http.MethodGet},
				Handle: func(context.Context, http.ResponseWriter, *http.Request) error {
					return nil
				},
			},
			expected: false,
		},
		{
			name: "missing-name",
			endpoint: httpbp.Endpoint{
				Methods: []string{http.MethodGet},
				Handle: func(context.Context, http.ResponseWriter, *http.Request) error {
					return nil
				},
			},
			expected: true,
		},
		{
			name: "missing-handle",
			endpoint: httpbp.Endpoint{
				Name:    "test",
				Methods: []string{http.MethodGet},
			},
			expected: true,
		},
		{
			name: "missing-methods",
			endpoint: httpbp.Endpoint{
				Name: "test",
				Handle: func(context.Context, http.ResponseWriter, *http.Request) error {
					return nil
				},
			},
			expected: true,
		},
		{
			name: "invalid-method",
			endpoint: httpbp.Endpoint{
				Name:    "test",
				Methods: []string{"foo"},
				Handle: func(context.Context, http.ResponseWriter, *http.Request) error {
					return nil
				},
			},
			expected: true,
		},
	}

	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				err := c.endpoint.Validate()
				if !c.expected && err != nil {
					t.Fatal(err)
				} else if c.expected && err == nil {
					t.Fatal("expected an error, got nil")
				}
			},
		)
	}
}

type mockEndpointRegistry struct {
	registry map[string]http.Handler
}

func (er *mockEndpointRegistry) init() {
	if er.registry == nil {
		er.registry = make(map[string]http.Handler)
	}
}

func (er *mockEndpointRegistry) Handle(pattern string, handler http.Handler) {
	er.init()
	er.registry[pattern] = handler
}

func (er *mockEndpointRegistry) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	er.init()
	h, ok := er.registry[r.URL.Path]
	if !ok {
		http.NotFound(w, r)
		return
	}
	h.ServeHTTP(w, r)
}

func TestServerArgsValidateAndSetDefaults(t *testing.T) {
	t.Parallel()

	store := newSecretsStore(t)
	defer store.Close()

	bp := baseplate.NewTestBaseplate(baseplate.NewTestBaseplateArgs{
		Config:          baseplate.Config{Addr: ":8080"},
		Store:           store,
		EdgeContextImpl: ecinterface.Mock(),
	})

	type expectation struct {
		args httpbp.ServerArgs
		err  bool
	}

	cases := []struct {
		name     string
		args     httpbp.ServerArgs
		expected expectation
	}{
		{
			name: "defaults",
			args: httpbp.ServerArgs{Baseplate: bp},
			expected: expectation{
				args: httpbp.ServerArgs{
					Baseplate:        bp,
					EndpointRegistry: http.NewServeMux(),
					TrustHandler:     httpbp.NeverTrustHeaders{},
				},
				err: false,
			},
		},
		{
			name: "defaults-dont-override",
			args: httpbp.ServerArgs{
				Baseplate:        bp,
				EndpointRegistry: &mockEndpointRegistry{},
				TrustHandler:     httpbp.AlwaysTrustHeaders{},
			},
			expected: expectation{
				args: httpbp.ServerArgs{
					Baseplate:        bp,
					EndpointRegistry: &mockEndpointRegistry{},
					TrustHandler:     httpbp.AlwaysTrustHeaders{},
				},
				err: false,
			},
		},
		{
			name:     "missing-baseplate",
			args:     httpbp.ServerArgs{},
			expected: expectation{err: true},
		},
		{
			name: "invalid-endpoint",
			args: httpbp.ServerArgs{
				Baseplate: bp,
				Endpoints: map[httpbp.Pattern]httpbp.Endpoint{"/foo": {}},
			},
			expected: expectation{err: true},
		},
	}

	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				args, err := c.args.ValidateAndSetDefaults()

				if !c.expected.err && err != nil {
					t.Error(err)
				} else if c.expected.err && err == nil {
					t.Error("expected an error, got nil")
				}

				if c.expected.err {
					return
				}

				if !reflect.DeepEqual(args, c.expected.args) {
					t.Errorf("ServerArgs mismatch, expected %#v, got %#v", c.expected.args, args)
				}
			},
		)
	}
}

func TestServerArgsSetupEndpoints(t *testing.T) {
	store := newSecretsStore(t)
	defer store.Close()

	bp := baseplate.NewTestBaseplate(baseplate.NewTestBaseplateArgs{
		Config:          baseplate.Config{Addr: ":8080"},
		Store:           store,
		EdgeContextImpl: ecinterface.Mock(),
	})

	t.Run(
		"validation-error",
		func(t *testing.T) {
			args := httpbp.ServerArgs{}
			if _, err := args.SetupEndpoints(); err == nil {
				t.Fatal("expected an error, got nil")
			}
		},
	)

	t.Run(
		"success",
		func(t *testing.T) {
			var pattern httpbp.Pattern = "/test"
			name := "test"
			recorder := edgecontextRecorder{}
			args := httpbp.ServerArgs{
				Baseplate: bp,
				Endpoints: map[httpbp.Pattern]httpbp.Endpoint{
					pattern: {
						Name:    name,
						Methods: []string{http.MethodGet},
						Handle: func(context.Context, http.ResponseWriter, *http.Request) error {
							return nil
						},
					},
				},
				EndpointRegistry: &mockEndpointRegistry{},
				Middlewares: []httpbp.Middleware{
					edgecontextRecorderMiddleware(ecinterface.Mock(), &recorder),
				},
				TrustHandler: httpbp.AlwaysTrustHeaders{},
				Logger:       log.TestWrapper(t),
			}

			args, err := args.SetupEndpoints()
			if err != nil {
				t.Fatal(err)
			}

			registry, ok := args.EndpointRegistry.(*mockEndpointRegistry)
			if !ok {
				t.Fatalf("registry changed types: %#v", registry)
			}

			if len(registry.registry) != 1 {
				t.Fatalf("registry does not have the expected number of Handlers: %#v", registry.registry)
			}

			handle, ok := registry.registry[string(pattern)]
			if !ok {
				t.Fatalf("no handler at %q: %#v", pattern, registry.registry)
			}

			req := newRequest(t, "foo")
			req.Method = http.MethodGet
			handle.ServeHTTP(httptest.NewRecorder(), req)

			// Test that the EdgeRequestContext midddleware was set up
			if recorder.header == "" {
				t.Error("edge request context not set")
			}
		},
	)
}

func TestNewTestBaseplateServer(t *testing.T) {
	type body struct {
		X int
		Y int
	}
	var pattern httpbp.Pattern = "/test"
	path := string(pattern)
	name := "test"
	expectedBody := body{X: 1, Y: 2}

	store := newSecretsStore(t)
	defer store.Close()

	bp := baseplate.NewTestBaseplate(baseplate.NewTestBaseplateArgs{
		Config:          baseplate.Config{Addr: ":8080"},
		Store:           store,
		EdgeContextImpl: ecinterface.Mock(),
	})

	c := counter{}
	args := httpbp.ServerArgs{
		Baseplate: bp,
		Endpoints: map[httpbp.Pattern]httpbp.Endpoint{
			pattern: {
				Name:    name,
				Methods: []string{http.MethodGet},
				Handle: func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
					return httpbp.WriteJSON(w, httpbp.Response{
						Body: expectedBody,
					})
				},
			},
		},
		Middlewares: []httpbp.Middleware{testMiddleware(&c)},
	}

	server, ts, err := httpbp.NewTestBaseplateServer(args)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	res, err := http.Get(ts.URL + path)
	if err != nil {
		t.Fatal(err)
	}

	var b body
	if err = json.NewDecoder(res.Body).Decode(&b); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(b, expectedBody) {
		t.Errorf("wrong response body, expected %#v, got %#v", expectedBody, b)
	}

	if c.count != 1 {
		t.Fatalf("Unexpected count value %v", c.count)
	}
}

func TestPanicRecovery(t *testing.T) {
	var pattern httpbp.Pattern = "/test"
	path := string(pattern)
	name := "test"

	store := newSecretsStore(t)
	defer store.Close()

	panicErr := fmt.Errorf("oops")

	bp := baseplate.NewTestBaseplate(baseplate.NewTestBaseplateArgs{
		Config:          baseplate.Config{Addr: ":8080"},
		Store:           store,
		EdgeContextImpl: ecinterface.Mock(),
	})

	args := httpbp.ServerArgs{
		Baseplate: bp,
		Endpoints: map[httpbp.Pattern]httpbp.Endpoint{
			pattern: {
				Name:    name,
				Methods: []string{http.MethodGet},
				Handle: func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
					panic(panicErr)
				},
			},
		},
		Middlewares: []httpbp.Middleware{
			func(name string, next httpbp.HandlerFunc) httpbp.HandlerFunc {
				return func(ctx context.Context, w http.ResponseWriter, r *http.Request) (err error) {
					var httpErr httpbp.HTTPError
					defer func() {
						if errors.As(err, &httpErr) {
							if httpErr.Response().Code != http.StatusInternalServerError {
								t.Errorf(
									"error code mismatch, expected %d, got %d",
									http.StatusInternalServerError,
									httpErr.Response().Code,
								)
							}
							if !errors.Is(httpErr, panicErr) {
								t.Errorf("expected http error to wrap %v, got %v", panicErr, httpErr.Unwrap())
							}
						} else {
							t.Fatalf("unexpected error type %v", err)
						}
					}()
					return next(ctx, w, r)
				}
			},
		},
	}

	server, ts, err := httpbp.NewTestBaseplateServer(args)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	res, err := http.Get(ts.URL + path)
	if err != nil {
		t.Fatal(err)
	}

	if res.StatusCode != http.StatusInternalServerError {
		t.Fatalf("unexpected service code")
	}
}
