package httpbp

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/reddit/baseplate.go"
	"github.com/reddit/baseplate.go/ecinterface"
	"github.com/reddit/baseplate.go/prometheusbp"
)

func TestPrometheusServerMetrics(t *testing.T) {
	testCases := []struct {
		name     string
		code     string
		success  string
		method   string
		endpoint string
	}{
		{
			name:     "success",
			code:     "OK",
			success:  "true",
			method:   "GET",
			endpoint: "/test",
		},
		{
			name:     "err",
			code:     "500 Internal Server Error",
			success:  "false",
			method:   "POST",
			endpoint: "/error",
		},
	}

	const (
		serverSlug = "testServer"
		get        = "GET"
		post       = "POST"
	)

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
						Handle:  func(ctx context.Context, w http.ResponseWriter, r *http.Request) error { return http.ErrHandlerTimeout },
					},
				},
				Middlewares: []Middleware{PrometheusServerMetrics(serverSlug, false)},
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
					t.Fatal("http.Get", err)
				}
			}

			if tt.method == post {
				_, err = client.Post(ts.URL+tt.endpoint, "", strings.NewReader("test"))
				if err != nil {
					t.Fatal("http.Get", err)
				}
			}
		})
	}
}
