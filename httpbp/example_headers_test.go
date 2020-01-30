package httpbp_test

import (
	"log"
	"net/http"

	"github.com/go-kit/kit/endpoint"
	httpgk "github.com/go-kit/kit/transport/http"

	"github.com/reddit/baseplate.go/httpbp"
)

// This example demonstrates how to use PopulateRequestContext
func ExamplePopulateRequestContext() {
	// variables should be properly initialized in production code
	var (
		IsHealthy              endpoint.Endpoint
		DecodeIsHealthyRequest httpgk.DecodeRequestFunc
		trustHandler           httpbp.NeverTrustHeaders
	)
	handler := http.NewServeMux()
	handler.Handle("/health", httpgk.NewServer(
		IsHealthy,
		DecodeIsHealthyRequest,
		httpbp.EncodeJSONResponse,
		httpgk.ServerBefore(
			httpbp.PopulateRequestContext(trustHandler),
		),
	))
	log.Fatal(http.ListenAndServe(":8080", handler))
}
