package httpbp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	retry "github.com/avast/retry-go"
	"github.com/reddit/baseplate.go/breakerbp"
	"github.com/reddit/baseplate.go/mqsend"
	"github.com/reddit/baseplate.go/tracing"
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
		b, err := ioutil.ReadAll(resp.Body)
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

		recorder := mqsend.OpenMockMessageQueue(mqsend.MessageQueueConfig{
			MaxQueueSize:   tracing.MaxQueueSize,
			MaxMessageSize: tracing.MaxSpanSize,
		})
		err := tracing.InitGlobalTracer(tracing.TracerConfig{
			SampleRate:               1,
			TestOnlyMockMessageQueue: recorder,
		})
		if err != nil {
			t.Fatal(err)
		}

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

		// MonitorClient is applied
		b, err := recorder.Receive(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		var span tracing.ZipkinSpan
		err = json.Unmarshal(b, &span)
		if err != nil {
			t.Fatal(err)
		}
		expected := "test.request"
		if span.Name != expected {
			t.Errorf("expected %s, actual: %q", expected, span.Name)
		}
	})
}

func TestNewClientConcurrency(t *testing.T) {
	var request uint64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddUint64(&request, 1)
		if atomic.LoadUint64(&request)%5 == 0 {
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

func TestMonitorClient(t *testing.T) {
	recorder := mqsend.OpenMockMessageQueue(mqsend.MessageQueueConfig{
		MaxQueueSize:   tracing.MaxQueueSize,
		MaxMessageSize: tracing.MaxSpanSize,
	})
	err := tracing.InitGlobalTracer(tracing.TracerConfig{
		SampleRate:               1,
		TestOnlyMockMessageQueue: recorder,
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	defer server.Close()

	middleware := MonitorClient("test")
	client := &http.Client{
		Transport: middleware(http.DefaultTransport),
	}
	_, err = client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	b, err := recorder.Receive(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var span tracing.ZipkinSpan
	err = json.Unmarshal(b, &span)
	if err != nil {
		t.Fatal(err)
	}

	expected := "test.request"
	if span.Name != expected {
		t.Errorf("expected %s, actual: %q", expected, span.Name)
	}
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
		b, err := ioutil.ReadAll(resp.Body)
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

func TestRetry(t *testing.T) {
	t.Run("retry for timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(2 * time.Millisecond)
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
			Timeout: time.Millisecond,
		}
		_, err := client.Get(server.URL)
		if err == nil {
			t.Fatalf("expected error to be non-nil")
		}
		expected := uint(2)
		if attempts != expected {
			t.Errorf("expected %d, actual: %d", expected, attempts)
		}
	})

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
		_, err := client.Get(server.URL)
		if err == nil {
			t.Fatalf("expected error to be non-nil")
		}
		expected := uint(2)
		if attempts != expected {
			t.Errorf("expected %d, actual: %d", expected, attempts)
		}
	})

	t.Run("retry POST request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			expected := "{}"
			got := string(b)
			if got != expected {
				t.Errorf("expected %q, got: %q", expected, got)
			}
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
		_, err := client.Post(server.URL, "application/json", bytes.NewBufferString("{}"))
		if err == nil {
			t.Fatalf("expected error to be non-nil")
		}
		expected := uint(2)
		if attempts != expected {
			t.Errorf("expected %d, actual: %d", expected, attempts)
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

	var errors uint64
	var wg sync.WaitGroup
	for i := 0; i < maxConcurrency*2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := client.Get(server.URL)
			if err != nil {
				atomic.AddUint64(&errors, 1)
				return
			}
			err = resp.Body.Close()
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()

	expected := uint64(maxConcurrency)
	if errors != expected {
		t.Errorf("expected %d, actual: %d", expected, errors)
	}
}
