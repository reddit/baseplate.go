package httpbp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/reddit/baseplate.go"
	"github.com/reddit/baseplate.go/ecinterface"
	"github.com/reddit/baseplate.go/prometheusbp"
)

type exampleRequest struct {
	Input string `json:"input"`
}

type exampleResponse struct {
	Message string `json:"message"`
}

func TestPrometheusClientServerMetrics(t *testing.T) {
	testCases := []struct {
		name     string
		code     string
		success  string
		method   string
		endpoint string
		size     int
	}{
		{
			name:     "success get",
			code:     "200",
			success:  "true",
			method:   http.MethodGet,
			endpoint: "/test",
		},
		{
			name:     "err post",
			code:     "401",
			success:  "false",
			method:   http.MethodPost,
			endpoint: "/error2",
			size:     16,
		},
		{
			name:     "internal err get",
			code:     "500",
			success:  "false",
			method:   http.MethodGet,
			endpoint: "/error",
		},
	}

	const serverSlug = "testServer"

	args := ServerArgs{
		Baseplate: baseplate.NewTestBaseplate(baseplate.NewTestBaseplateArgs{
			Config:          baseplate.Config{Addr: ":8080"},
			EdgeContextImpl: ecinterface.Mock(),
		}),
		Endpoints: map[Pattern]Endpoint{
			"/test": {
				Name:    "test",
				Methods: []string{http.MethodGet},
				Handle:  func(ctx context.Context, w http.ResponseWriter, r *http.Request) error { return nil },
			},
			"/error2": {
				Name:    "error",
				Methods: []string{http.MethodPost},
				Handle: func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
					var req exampleRequest
					if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
						return fmt.Errorf("decoding %T: %w", req, err)
					}
					body := exampleResponse{
						Message: fmt.Sprintf("Input: %q", req.Input),
					}
					return WriteJSON(w, Response{Body: body, Code: Unauthorized().code})
				},
			},
			"/error": {
				Name:    "error",
				Methods: []string{http.MethodGet},
				Handle: func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
					return errors.New("test")
				},
			},
		},
		Middlewares: []Middleware{PrometheusServerMetrics(serverSlug)},
	}

	server, ts, err := NewTestBaseplateServer(args)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	middleware := PrometheusClientMetrics(serverSlug)
	client := &http.Client{
		Transport: middleware(http.DefaultTransport),
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			serverLatency.Reset()
			serverTotalRequests.Reset()
			serverActiveRequests.Reset()
			clientLatency.Reset()
			clientTotalRequests.Reset()
			clientActiveRequests.Reset()

			serverTotalRequestLabels := []string{
				tt.method,
				tt.success,
				tt.endpoint,
				tt.code,
			}

			serverActiveRequestLabels := []string{
				tt.method,
				tt.endpoint,
			}

			serverSizeLabels := []string{
				tt.method,
				tt.success,
				tt.endpoint,
			}

			totalRequestLabels := []string{
				tt.method,
				tt.success,
				tt.endpoint,
				tt.code,
				serverSlug,
			}

			activeRequestLabels := []string{
				tt.method,
				tt.endpoint,
				serverSlug,
			}

			clientSizeLabels := []string{
				tt.method,
				tt.success,
				tt.endpoint,
				serverSlug,
			}

			defer prometheusbp.MetricTest(t, "server latency", serverLatency, serverSizeLabels...).CheckExists()
			defer prometheusbp.MetricTest(t, "server total requests", serverTotalRequests, serverTotalRequestLabels...).CheckDelta(1)
			defer prometheusbp.MetricTest(t, "server active requests", serverActiveRequests, serverActiveRequestLabels...).CheckDelta(0)
			defer prometheusbp.MetricTest(t, "server request size", serverRequestSize, serverSizeLabels...).CheckDelta(float64(tt.size))
			defer prometheusbp.MetricTest(t, "server response size", serverResponseSize, serverSizeLabels...).CheckDelta(0)
			defer prometheusbp.MetricTest(t, "client latency", clientLatency, clientSizeLabels...).CheckExists()
			defer prometheusbp.MetricTest(t, "client total requests", clientTotalRequests, totalRequestLabels...).CheckDelta(1)
			defer prometheusbp.MetricTest(t, "client active requests", clientActiveRequests, activeRequestLabels...).CheckDelta(0)

			if tt.method == http.MethodGet {
				_, err = client.Get(ts.URL + tt.endpoint)
				if err != nil {
					t.Fatal("client.Get", err)
				}
			}

			if tt.method == http.MethodPost {
				input := exampleRequest{Input: "foo"}
				var body bytes.Buffer
				if err := json.NewEncoder(&body).Encode(input); err != nil {
					t.Fatal(err)
				}

				_, err := client.Post(ts.URL+tt.endpoint, "", &body)
				if err != nil {
					t.Fatal("client.Post", err)
				}
			}
		})
	}
}
