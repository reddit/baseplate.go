package httpbp

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/avast/retry-go"
	"github.com/sony/gobreaker"

	"github.com/reddit/baseplate.go/breakerbp"
	"github.com/reddit/baseplate.go/internal/faults"
)

func TestNewClient(t *testing.T) {
	t.Run("get request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "foo")
		}))
		defer server.Close()

		client, err := NewClient(ClientConfig{
			Slug: "test",
		})
		if err != nil {
			t.Fatal(err)
		}

		resp, err := client.Get(server.URL)
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		actual := string(b)
		expected := "foo"
		if actual != expected {
			t.Errorf("expected %q, actual: %q", expected, actual)
		}
	})

	t.Run("default middlewares are applied", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		client, err := NewClient(ClientConfig{
			Slug: "test",
		})
		if err != nil {
			t.Fatalf("NewClient returned error: %v", err)
		}

		// ClientErrorWrapper is applied
		_, err = client.Get(server.URL)
		if err == nil {
			t.Fatal("expected error but is nil")
		}
		var e *ClientError
		if !errors.As(err, &e) {
			t.Errorf("expected error wrap error of type %T", *e)
		}
	})
}

func TestNewClientConcurrency(t *testing.T) {
	var request atomic.Uint64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request.Add(1)
		if request.Load()%5 == 0 {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			io.WriteString(w, "foo")
		}
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Slug:              "test",
		MaxConnections:    10,
		MaxErrorReadAhead: DefaultMaxErrorReadAhead,
		RetryOptions: []retry.Option{
			retry.Attempts(3),
		},
		CircuitBreaker: &breakerbp.Config{
			MinRequestsToTrip: 2,
			FailureThreshold:  0.5,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			resp, err := client.Get(server.URL)
			if err != nil {
				return
			}
			err = resp.Body.Close()
			if err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
}

func TestClientErrorWrapper(t *testing.T) {
	t.Run("HTTP 200", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "foo")
		}))
		defer server.Close()

		client := &http.Client{
			Transport: ClientErrorWrapper(DefaultMaxErrorReadAhead)(http.DefaultTransport),
		}
		resp, err := client.Get(server.URL)
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		expected := "foo"
		actual := string(b)
		if expected != actual {
			t.Errorf("expected %q, actual: %q", expected, actual)
		}
	})

	t.Run("HTTP 500", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		client := &http.Client{
			Transport: ClientErrorWrapper(DefaultMaxErrorReadAhead)(http.DefaultTransport),
		}
		_, err := client.Get(server.URL)
		if err == nil {
			t.Fatal("expected error but is nil")
		}
		var e *ClientError
		if !errors.As(err, &e) {
			t.Errorf("expected error wrap error of type %T", *e)
		}
	})

	t.Run("reads error response body up to a limit", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, `{"error": "request failed because…}`)
		}))
		defer server.Close()

		maxErrorReadAhead := 30
		client := &http.Client{
			Transport: ClientErrorWrapper(maxErrorReadAhead)(http.DefaultTransport),
		}
		_, err := client.Get(server.URL)
		if err == nil {
			t.Fatal("expected error but is nil")
		}
		clientError := errors.Unwrap(err).(*ClientError)
		expected := `{"error": "request failed beca`
		if clientError.AdditionalInfo != expected {
			t.Errorf("expected %v, actual: %v", expected, clientError.AdditionalInfo)
		}
	})
}

func unwrapRetryErrors(err error) []error {
	var errs interface {
		error

		Unwrap() []error
	}
	if errors.As(err, &errs) {
		return errs.Unwrap()
	}
	return []error{err}
}

func TestRetry(t *testing.T) {
	t.Run("retry for HTTP 500", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		var attempts uint
		client := &http.Client{
			Transport: Retries(
				DefaultMaxErrorReadAhead,
				retry.Attempts(2),
				retry.OnRetry(func(n uint, err error) {
					// set number of attempts to check if retries were attempted
					attempts = n + 1
				}),
			)(http.DefaultTransport),
		}
		u, err := url.Parse(server.URL)
		if err != nil {
			t.Fatalf("Failed to parse url %q: %v", server.URL, err)
		}
		req := &http.Request{
			Method: http.MethodPost,
			URL:    u,

			// Explicitly set Body to http.NoBody and GetBody to nil,
			// This request should not cause Retries middleware to be skipped.
			Body:    http.NoBody,
			GetBody: nil,
		}
		_, err = client.Do(req)
		if err == nil {
			t.Fatalf("expected error to be non-nil")
		}
		expected := uint(2)
		if attempts != expected {
			t.Errorf("expected %d, actual: %d", expected, attempts)
		}
		errs := unwrapRetryErrors(err)
		if len(errs) != int(expected) {
			t.Errorf("Expected %d retry erros, got %+v", expected, errs)
		}
		for i, err := range errs {
			var ce *ClientError
			if errors.As(err, &ce) {
				if got, want := ce.StatusCode, http.StatusInternalServerError; got != want {
					t.Errorf("#%d: status got %d want %d", i, got, want)
				}
			} else {
				t.Errorf("#%d: %#v is not of type *httpbp.ClientError", i, err)
			}
		}
	})

	t.Run("retry POST+HTTPS request", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			expected := "{}"
			got := string(b)
			if got != expected {
				t.Errorf("expected %q, got: %q", expected, got)
			}
			t.Logf("Full body: %q", got)
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		var attempts uint
		t.Log(server.URL)
		client := server.Client()
		client.Transport = Retries(
			DefaultMaxErrorReadAhead,
			retry.Attempts(2),
			retry.OnRetry(func(n uint, err error) {
				// set number of attempts to check if retries were attempted
				attempts = n + 1
			}),
		)(client.Transport)
		_, err := client.Post(server.URL, "application/json", bytes.NewBufferString("{}"))
		if err == nil {
			t.Fatalf("expected error to be non-nil")
		}
		expected := uint(2)
		if attempts != expected {
			t.Errorf("expected %d, actual: %d", expected, attempts)
		}
		errs := unwrapRetryErrors(err)
		if len(errs) != int(expected) {
			t.Errorf("Expected %d retry erros, got %+v", expected, errs)
		}
		for i, err := range errs {
			var ce *ClientError
			if errors.As(err, &ce) {
				if got, want := ce.StatusCode, http.StatusInternalServerError; got != want {
					t.Errorf("#%d: status got %d want %d", i, got, want)
				}
			} else {
				t.Errorf("#%d: %#v is not of type *httpbp.ClientError", i, err)
			}
		}
	})

	t.Run("skip retry for wrongly constructed request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		client := &http.Client{
			Transport: Retries(
				DefaultMaxErrorReadAhead,
				retry.Attempts(2),
				retry.OnRetry(func(n uint, err error) {
					t.Errorf("Retry not skipped. OnRetry called with (%d, %v)", n, err)
				}),
			)(http.DefaultTransport),
		}
		req, err := http.NewRequest(http.MethodGet, server.URL, bytes.NewBufferString("{}"))
		if err != nil {
			t.Fatalf("Failed to create http request: %v", err)
		}
		req.GetBody = nil
		if _, err := client.Do(req); err == nil {
			t.Fatalf("expected error to be non-nil")
		}
	})
}

func TestMaxConcurrency(t *testing.T) {
	var maxConcurrency = 10

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	client := &http.Client{
		Transport: MaxConcurrency(int64(maxConcurrency))(http.DefaultTransport),
	}

	var errors atomic.Uint64
	var wg sync.WaitGroup
	for i := 0; i < maxConcurrency*2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := client.Get(server.URL)
			if err != nil {
				errors.Add(1)
				return
			}
			err = resp.Body.Close()
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()

	if got, want := errors.Load(), uint64(maxConcurrency); got != want {
		t.Errorf("Number of errors got %d want %d", got, want)
	}
}

func TestCircuitBreaker(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const code = http.StatusInternalServerError
		w.WriteHeader(code)
		io.WriteString(w, http.StatusText(code))
	}))
	// The first 2 requests should return normal error (ClientError),
	// the 3rd one should return breaker error.
	breaker := CircuitBreaker(breakerbp.Config{
		MinRequestsToTrip: 2,
		FailureThreshold:  1,
	})
	client := server.Client()
	client.Transport = breaker(client.Transport)

	_, err := client.Get(server.URL)
	if !errors.As(err, new(ClientError)) {
		t.Errorf("Expected the first request to return ClientError, got %v", err)
	}

	_, err = client.Get(server.URL)
	if !errors.As(err, new(ClientError)) {
		t.Errorf("Expected the second request to return ClientError, got %v", err)
	}

	_, err = client.Get(server.URL)
	if !errors.Is(err, gobreaker.ErrOpenState) {
		t.Errorf("Expected the third request to return %v, got %v", gobreaker.ErrOpenState, err)
	}
}

func TestFaultInjection(t *testing.T) {
	testCases := []struct {
		name                       string
		faultServerAddrMatch       bool
		faultServerMethodHeader    string
		faultDelayMsHeader         string
		faultDelayPercentageHeader string
		faultAbortCodeHeader       string
		faultAbortMessageHeader    string
		faultAbortPercentageHeader string

		wantResp *http.Response
	}{
		{
			name: "no fault specified",
			wantResp: &http.Response{
				StatusCode: http.StatusOK,
			},
		},
		{
			name: "abort",

			faultServerAddrMatch:    true,
			faultServerMethodHeader: "testMethod",
			faultAbortCodeHeader:    "500",

			wantResp: &http.Response{
				StatusCode: http.StatusInternalServerError,
			},
		},
		{
			name: "service does not match",

			faultServerAddrMatch:    false,
			faultServerMethodHeader: "testMethod",
			faultAbortCodeHeader:    "500",

			wantResp: &http.Response{
				StatusCode: http.StatusOK,
			},
		},
		{
			name: "method does not match",

			faultServerAddrMatch:    true,
			faultServerMethodHeader: "fooMethod",
			faultAbortCodeHeader:    "500",

			wantResp: &http.Response{
				StatusCode: http.StatusOK,
			},
		},
		{
			name: "less than min abort code",

			faultServerAddrMatch:    true,
			faultServerMethodHeader: "testMethod",
			faultAbortCodeHeader:    "99",

			wantResp: &http.Response{
				StatusCode: http.StatusOK,
			},
		},
		{
			name: "greater than max abort code",

			faultServerAddrMatch:    true,
			faultServerMethodHeader: "testMethod",
			faultAbortCodeHeader:    "600",

			wantResp: &http.Response{
				StatusCode: http.StatusOK,
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				io.WriteString(w, "Success!")
			}))
			defer server.Close()

			client, err := NewClient(ClientConfig{
				Slug: "test",
			})
			if err != nil {
				t.Fatalf("NewClient returned error: %v", err)
			}

			req, err := http.NewRequest("GET", server.URL+"/testMethod", nil)
			if err != nil {
				t.Fatalf("unexpected error when creating request: %v", err)
			}

			if tt.faultServerAddrMatch {
				// We can't set a specific address here because the middleware
				// relies on the DNS address, which is not customizable when making
				// real requests to a local HTTP test server.
				parsed, err := url.Parse(server.URL)
				if err != nil {
					t.Fatalf("unexpected error when parsing httptest server URL: %v", err)
				}
				req.Header.Set(faults.FaultServerAddressHeader, parsed.Hostname())
			}
			if tt.faultServerMethodHeader != "" {
				req.Header.Set(faults.FaultServerMethodHeader, tt.faultServerMethodHeader)
			}
			if tt.faultDelayMsHeader != "" {
				req.Header.Set(faults.FaultDelayMsHeader, tt.faultDelayMsHeader)
			}
			if tt.faultDelayPercentageHeader != "" {
				req.Header.Set(faults.FaultDelayPercentageHeader, tt.faultDelayPercentageHeader)
			}
			if tt.faultAbortCodeHeader != "" {
				req.Header.Set(faults.FaultAbortCodeHeader, tt.faultAbortCodeHeader)
			}
			if tt.faultAbortMessageHeader != "" {
				req.Header.Set(faults.FaultAbortMessageHeader, tt.faultAbortMessageHeader)
			}
			if tt.faultAbortPercentageHeader != "" {
				req.Header.Set(faults.FaultAbortPercentageHeader, tt.faultAbortPercentageHeader)
			}

			resp, err := client.Do(req)

			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tt.wantResp.StatusCode != resp.StatusCode {
				t.Fatalf("expected response code %v, got %v", tt.wantResp.StatusCode, resp.StatusCode)
			}
		})
	}
}
