package httpbp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/reddit/baseplate.go"
	"github.com/reddit/baseplate.go/ecinterface"
	"github.com/reddit/baseplate.go/prometheusbp/promtest"
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

			serverSizeLabels := prometheus.Labels{
				methodLabel:   tt.method,
				successLabel:  tt.success,
				endpointLabel: tt.endpoint,
			}

			serverTotalRequestLabels := prometheus.Labels{
				methodLabel:   tt.method,
				successLabel:  tt.success,
				endpointLabel: tt.endpoint,
				codeLabel:     tt.code,
			}

			serverActiveRequestLabels := prometheus.Labels{
				methodLabel:   tt.method,
				endpointLabel: tt.endpoint,
			}

			clientLatencyLabels := prometheus.Labels{
				methodLabel:            tt.method,
				successLabel:           tt.success,
				endpointLabel:          tt.endpoint,
				remoteServiceSlugLabel: serverSlug,
			}

			clientTotalRequestLabels := prometheus.Labels{
				methodLabel:            tt.method,
				successLabel:           tt.success,
				endpointLabel:          tt.endpoint,
				codeLabel:              tt.code,
				remoteServiceSlugLabel: serverSlug,
			}

			clientActiveRequestLabels := prometheus.Labels{
				methodLabel:            tt.method,
				endpointLabel:          tt.endpoint,
				remoteServiceSlugLabel: serverSlug,
			}

			defer promtest.NewPrometheusMetricTest(t, "server latency", serverLatency, serverSizeLabels).CheckExists()
			defer promtest.NewPrometheusMetricTest(t, "server total requests", serverTotalRequests, serverTotalRequestLabels).CheckDelta(1)
			defer promtest.NewPrometheusMetricTest(t, "server active requests", serverActiveRequests, serverActiveRequestLabels).CheckDelta(0)
			defer promtest.NewPrometheusMetricTest(t, "server request size", serverRequestSize, serverSizeLabels).CheckDelta(float64(tt.size))
			defer promtest.NewPrometheusMetricTest(t, "server response size", serverResponseSize, serverSizeLabels).CheckDelta(0)
			defer promtest.NewPrometheusMetricTest(t, "client latency", clientLatency, clientLatencyLabels).CheckExists()
			defer promtest.NewPrometheusMetricTest(t, "client total requests", clientTotalRequests, clientTotalRequestLabels).CheckDelta(1)
			defer promtest.NewPrometheusMetricTest(t, "client active requests", clientActiveRequests, clientActiveRequestLabels).CheckDelta(0)

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

				if _, err := client.Post(ts.URL+tt.endpoint, "", &body); err != nil {
					t.Fatal("client.Post", err)
				}
			}
		})
	}
}