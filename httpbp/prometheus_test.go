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

type exampleEndpointRequest struct {
	Input string `json:"input"`
}

type exampleEndpointResponse struct {
	Message string `json:"message"`
}

func TestPrometheusServerMetrics(t *testing.T) {
	const (
		serverSlug = "testServer"
		get        = "GET"
		post       = "POST"
	)
	testCases := []struct {
		name     string
		code     string
		success  string
		method   string
		endpoint string
	}{
		{
			name:     "success",
			code:     "200",
			success:  "true",
			method:   get,
			endpoint: "/test",
		},
		{
			name:     "err get",
			code:     "500",
			success:  "false",
			method:   get,
			endpoint: "/error2",
		},
		{
			name:     "err post",
			code:     "401",
			success:  "false",
			method:   post,
			endpoint: "/error",
		},
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

			sizeLabels := []string{
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

			defer prometheusbp.MetricTest(t, "server latency", serverLatency).CheckExists()
			defer prometheusbp.MetricTest(t, "server total requests", serverTotalRequests, serverTotalRequestLabels...).CheckDelta(1)
			defer prometheusbp.MetricTest(t, "server active requests", serverActiveRequests, serverActiveRequestLabels...).CheckDelta(0)
			defer prometheusbp.MetricTest(t, "server request size", serverRequestSize, sizeLabels...).CheckDelta(0)
			defer prometheusbp.MetricTest(t, "server response size", serverResponseSize, sizeLabels...).CheckDelta(0)
			defer prometheusbp.MetricTest(t, "client latency", clientLatency).CheckNotExists()
			defer prometheusbp.MetricTest(t, "client total requests", clientTotalRequests, totalRequestLabels...).CheckDelta(0)
			defer prometheusbp.MetricTest(t, "client active requests", clientActiveRequests, activeRequestLabels...).CheckDelta(0)

			var methods = []string{}
			switch {
			case tt.method == get:
				methods = append(methods, get)
			case tt.method == post:
				methods = append(methods, post)
			}
			args := ServerArgs{
				Baseplate: baseplate.NewTestBaseplate(baseplate.NewTestBaseplateArgs{
					Config:          baseplate.Config{Addr: ":8080"},
					EdgeContextImpl: ecinterface.Mock(),
				}),
				Endpoints: map[Pattern]Endpoint{
					"/test": {
						Name:    "test",
						Methods: methods,
						Handle:  func(ctx context.Context, w http.ResponseWriter, r *http.Request) error { return nil },
					},
					"/error": {
						Name:    "error",
						Methods: methods,
						Handle: func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
							var req exampleEndpointRequest
							if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
								return fmt.Errorf("decoding %T: %w", req, err)
							}
							body := exampleEndpointResponse{
								Message: fmt.Sprintf("Input: %q", req.Input),
							}
							return WriteJSON(w, Response{Body: body, Code: http.StatusUnauthorized})
						},
					},
					"/error2": {
						Name:    "error",
						Methods: methods,
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

			client := &http.Client{
				Transport: http.DefaultTransport,
			}

			if tt.method == get {
				_, err = client.Get(ts.URL + tt.endpoint)
				if err != nil {
					t.Fatal("client.Get", err)
				}
			}

			if tt.method == post {
				input := exampleEndpointRequest{Input: "foo"}
				var body bytes.Buffer
				if err := json.NewEncoder(&body).Encode(input); err != nil {
					t.Fatal(err)
				}
				_, err = client.Post(ts.URL+tt.endpoint, "", &body)
				if err != nil {
					t.Fatal("client.Post", err)
				}
			}
		})
	}
}
