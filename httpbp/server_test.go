package httpbp_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/reddit/baseplate.go/headerbp"

	"github.com/reddit/baseplate.go"
	"github.com/reddit/baseplate.go/ecinterface"
	"github.com/reddit/baseplate.go/httpbp"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/secrets"
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

func TestBaseplateHeaderPropagation(t *testing.T) {
	expectedHeaders := map[string][]string{
		"x-bp-from-edge": {"true"},
		"x-bp-test":      {"foo"},
	}
	store, _, err := secrets.NewTestSecrets(context.TODO(), map[string]secrets.GenericSecret{
		"secret/baseplate/headerbp/signature-key": {
			Type:     secrets.VersionedType,
			Current:  "dGVzdA==", // test
			Encoding: secrets.Base64Encoding,
		},
	})
	if err != nil {
		t.Fatalf("failed to create test secrets: %v", err)
	}
	t.Cleanup(func() {
		store.Close()
	})
	downstreamBP := baseplate.NewTestBaseplate(baseplate.NewTestBaseplateArgs{
		Config: baseplate.Config{
			Addr: ":8081",
		},
		Store:           store,
		EdgeContextImpl: ecinterface.Mock(),
	})
	downstreamServer, err := httpbp.NewBaseplateServer(httpbp.ServerArgs{
		Baseplate: downstreamBP,
		Endpoints: map[httpbp.Pattern]httpbp.Endpoint{
			"/say-hello": {
				Name:    "say-hello",
				Methods: []string{http.MethodGet},
				Handle: func(ctx context.Context, writer http.ResponseWriter, request *http.Request) error {
					for wantKey, wantValue := range expectedHeaders {
						if v := request.Header.Values(wantKey); len(v) == 0 {
							t.Fatalf("missing header %q", wantKey)
						} else if diff := cmp.Diff(v, wantValue, cmpopts.SortSlices(func(a, b string) bool {
							return a < b
						})); diff != "" {
							t.Fatalf("header %q values mismatch (-want +got):\n%s", wantKey, diff)
						}
					}
					return nil
				},
			},
		},
		Middlewares: []httpbp.Middleware{
			httpbp.ServerBaseplateHeadersMiddleware("originHTTPBPV0", store, "secret/baseplate/headerbp/signature-key"),
		},
	})
	if err != nil {
		t.Fatalf("failed to create test downstreamServer: %v", err)
	}
	t.Cleanup(func() {
		downstreamServer.Close()
	})
	go downstreamServer.Serve()
	time.Sleep(100 * time.Millisecond) // wait for the server to start

	downstreamBaseURL, err := url.Parse("http://" + downstreamServer.Baseplate().GetConfig().Addr + "/")
	if err != nil {
		t.Fatalf("failed to parse test originServer base URL: %v", err)
	}

	downstreamClient, err := httpbp.NewClient(
		httpbp.ClientConfig{
			Slug:                   "downstreamHTTPBPV0",
			SecretsStore:           store,
			HeaderbpSigningKeyPath: "secret/baseplate/headerbp/signature-key",
		},
		// This is to test that the client middleware is idempotent
		httpbp.ClientBaseplateHeadersMiddleware("downstream", store, "secret/baseplate/headerbp/signature-key"),
	)
	if err != nil {
		t.Fatalf("failed to create test client: %v", err)
	}

	originBP := baseplate.NewTestBaseplate(baseplate.NewTestBaseplateArgs{
		Config: baseplate.Config{
			Addr: ":8082",
		},
		Store:           store,
		EdgeContextImpl: ecinterface.Mock(),
	})
	originServer, err := httpbp.NewBaseplateServer(httpbp.ServerArgs{
		Baseplate: originBP,
		Endpoints: map[httpbp.Pattern]httpbp.Endpoint{
			"/say-hello": {
				Name:    "say-hello",
				Methods: []string{http.MethodGet},
				Handle: func(ctx context.Context, writer http.ResponseWriter, request *http.Request) error {
					for wantKey, wantValue := range expectedHeaders {
						if v := request.Header.Values(wantKey); len(v) == 0 {
							t.Fatalf("missing header %q", wantKey)
						} else if diff := cmp.Diff(v, wantValue, cmpopts.SortSlices(func(a, b string) bool {
							return a < b
						})); diff != "" {
							t.Fatalf("header %q values mismatch (-want +got):\n%s", wantKey, diff)
						}
					}

					req, err := http.NewRequestWithContext(
						ctx,
						http.MethodGet,
						downstreamBaseURL.JoinPath("say-hello").String(),
						nil,
					)
					if err != nil {
						t.Fatalf("creating request: %v", err)
					}

					resp, err := downstreamClient.Do(req)
					if err != nil {
						t.Fatalf("unexpected error: %v", err)
					}
					if resp.StatusCode != http.StatusOK {
						t.Fatalf("unexpected status code: %d", resp.StatusCode)
					}

					invalidReq, err := http.NewRequestWithContext(
						ctx,
						http.MethodGet,
						downstreamBaseURL.JoinPath("say-hello").String(),
						nil,
					)
					if err != nil {
						t.Fatalf("creating request: %v", err)
					}
					invalidReq.Header.Set("x-bp-test", "bar")

					if _, err := downstreamClient.Do(req); !errors.Is(err, headerbp.ErrNewInternalHeaderNotAllowed) {
						t.Fatalf("error mismatch, want %v, got %v", headerbp.ErrNewInternalHeaderNotAllowed, err)
					}
					return nil
				},
			},
		},
		Middlewares: []httpbp.Middleware{
			httpbp.ServerBaseplateHeadersMiddleware("originHTTPBPV0", store, "secret/baseplate/headerbp/signature-key"),
		},
	})
	if err != nil {
		t.Fatalf("failed to create test originServer: %v", err)
	}
	t.Cleanup(func() {
		originServer.Close()
	})
	go originServer.Serve()
	time.Sleep(100 * time.Millisecond) // wait for the server to start

	baseURL, err := url.Parse("http://" + originServer.Baseplate().GetConfig().Addr + "/")
	if err != nil {
		t.Fatalf("failed to parse test originServer base URL: %v", err)
	}

	client, err := httpbp.NewClient(
		httpbp.ClientConfig{
			Slug: "downstreamHTTPBPV0",
		},
	)
	if err != nil {
		t.Fatalf("failed to create test client: %v", err)
	}

	req, err := http.NewRequest(
		http.MethodGet,
		baseURL.JoinPath("/say-hello").String(),
		nil,
	)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	var headerNames []string
	for name, values := range expectedHeaders {
		headerNames = append(headerNames, name)
		req.Header.Set(name, values[0])
	}
	secret, err := store.GetVersionedSecret("secret/baseplate/headerbp/signature-key")
	if err != nil {
		t.Fatalf("failed to get secret: %v", err)
	}
	signature, err := headerbp.SignHeaders(req.Context(), secret, headerNames, req.Header.Get)
	if err != nil {
		t.Fatalf("failed to sign headers: %v", err)
	}
	req.Header.Set(headerbp.SignatureHeaderCanonicalHTTP, signature)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status code: %d", resp.StatusCode)
	}
}

func TestBaseplateHeaderPropagation_untrusted(t *testing.T) {
	expectedHeaders := map[string][]string{
		"x-bp-from-edge": {"true"},
		"x-bp-test":      {"foo"},
	}
	store, _, err := secrets.NewTestSecrets(context.TODO(), nil)
	if err != nil {
		t.Fatalf("failed to create test secrets: %v", err)
	}
	t.Cleanup(func() {
		store.Close()
	})
	bp := baseplate.NewTestBaseplate(baseplate.NewTestBaseplateArgs{
		Config: baseplate.Config{
			Addr: ":8081",
		},
		Store:           store,
		EdgeContextImpl: ecinterface.Mock(),
	})

	originServer, err := httpbp.NewBaseplateServer(httpbp.ServerArgs{
		Baseplate: bp,
		Endpoints: map[httpbp.Pattern]httpbp.Endpoint{
			"/say-hello": {
				Name:    "say-hello",
				Methods: []string{http.MethodGet},
				Handle: func(ctx context.Context, writer http.ResponseWriter, request *http.Request) error {
					for wantKey := range expectedHeaders {
						if v := request.Header.Values(wantKey); len(v) != 0 {
							t.Fatalf("expected no values for header %q, got %+v", wantKey, v)
						}
					}

					untrusted, ok := httpbp.GetUntrustedBaseplateHeaders(ctx)
					if !ok {
						t.Fatal("expected untrusted headers")
					}
					for wantKey, wantValue := range expectedHeaders {
						if v, ok := untrusted[wantKey]; !ok {
							t.Fatalf("missing header %q", wantKey)
						} else if diff := cmp.Diff(v, wantValue[0]); diff != "" {
							t.Fatalf("header %q values mismatch (-want +got):\n%s", wantKey, diff)
						}
					}

					return nil
				},
			},
		},
		Middlewares: []httpbp.Middleware{
			httpbp.ServerBaseplateHeadersMiddleware("originHTTPBPV0", store, "secret/baseplate/headerbp/signature-key"),
		},
	})
	if err != nil {
		t.Fatalf("failed to create test originServer: %v", err)
	}
	t.Cleanup(func() {
		originServer.Close()
	})
	go originServer.Serve()
	time.Sleep(100 * time.Millisecond) // wait for the server to start

	baseURL, err := url.Parse("http://" + originServer.Baseplate().GetConfig().Addr + "/")
	if err != nil {
		t.Fatalf("failed to parse test originServer base URL: %v", err)
	}

	client, err := httpbp.NewClient(
		httpbp.ClientConfig{
			Slug: "downstreamHTTPBPV0",
		},
		withBaseURL(baseURL),
	)
	if err != nil {
		t.Fatalf("failed to create test client: %v", err)
	}

	req, err := http.NewRequest(
		http.MethodGet,
		baseURL.JoinPath("say-hello").String(),
		nil,
	)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	for name, values := range expectedHeaders {
		req.Header.Set(name, values[0])
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status code: %d", resp.StatusCode)
	}
}

func withBaseURL(baseURL *url.URL) httpbp.ClientMiddleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(req *http.Request) (resp *http.Response, err error) {
			resolved := req.Clone(req.Context())
			resolved.URL = baseURL.ResolveReference(req.URL)
			return next.RoundTrip(resolved)
		})
	}
}

type roundTripperFunc func(req *http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
