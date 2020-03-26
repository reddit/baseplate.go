package edgecontext_test

import (
	"net/http"

	"github.com/go-kit/kit/endpoint"
	httpgk "github.com/go-kit/kit/transport/http"

	"github.com/reddit/baseplate.go/edgecontext"
	"github.com/reddit/baseplate.go/httpbp"
	"github.com/reddit/baseplate.go/log"
)

// This example shows how to use the InjectHTTPEdgeContext middleware in a
// go-kit HTTP service.
func ExampleInjectHTTPEdgeContext() {
	// variables should be properly initialized in production code
	var (
		IsHealthy              endpoint.Endpoint
		DecodeIsHealthyRequest httpgk.DecodeRequestFunc
		trustHandler           httpbp.AlwaysTrustHeaders
		logger                 log.Wrapper
		impl                   *edgecontext.Impl
	)
	handler := http.NewServeMux()
	handler.Handle("/health", httpgk.NewServer(
		// You don't have to use endpoint.Chain when using a single
		// endpoint.Middleware, as we are in this example, but it will make it
		// easier to add more later on down the line (since you'll have to add
		// it then anyways).
		endpoint.Chain(
			edgecontext.InjectHTTPEdgeContext(impl, logger),
		)(IsHealthy),
		DecodeIsHealthyRequest,
		httpbp.EncodeJSONResponse,
		httpgk.ServerBefore(
			// InjectHTTPEdgeContext relies on PopulateRequestContext from the
			// httpbp package to set up the context object with the appropriate
			// headers from the request.
			httpbp.PopulateRequestContext(trustHandler),
		),
	))
	log.Fatal(http.ListenAndServe(":8080", handler))
}
