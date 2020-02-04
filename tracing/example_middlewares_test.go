package tracing_test

import (
	"net/http"

	"github.com/go-kit/kit/endpoint"
	httpgk "github.com/go-kit/kit/transport/http"

	"github.com/reddit/baseplate.go/httpbp"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/tracing"
)

// This example shows how to use the InjectHTTPServerSpan middleware in a
// go-kit HTTP service.
func ExampleInjectHTTPServerSpan() {
	// variables should be properly initialized in production code
	var (
		IsHealthy              endpoint.Endpoint
		DecodeIsHealthyRequest httpgk.DecodeRequestFunc
		trustHandler           httpbp.AlwaysTrustHeaders
	)
	name := "my-service"
	handler := http.NewServeMux()
	handler.Handle("/health", httpgk.NewServer(
		// You don't have to use endpoint.Chain when using a single
		// endpoint.Middleware, as we are in this example, but it will make it
		// easier to add more later on down the line (since you'll have to add
		// it then anyways).
		endpoint.Chain(
			tracing.InjectHTTPServerSpan(name),
		)(IsHealthy),
		DecodeIsHealthyRequest,
		httpbp.EncodeJSONResponse,
		httpgk.ServerBefore(
			// InjectHTTPServerSpan relies on PopulateRequestContext from the
			// httpbp package to set up the context object with the appropriate
			// headers from the request.
			httpbp.PopulateRequestContext(trustHandler),
		),
	))
	log.Fatal(http.ListenAndServe(":8080", handler))
}
