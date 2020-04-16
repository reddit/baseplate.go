package httpbp_test

import (
	"context"
	"errors"
	"net/http"

	"github.com/reddit/baseplate.go/edgecontext"
	"github.com/reddit/baseplate.go/httpbp"
	"github.com/reddit/baseplate.go/log"
)

type body struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type errorResponse struct {
	Reason  string `json:"reason"`
	Message string `json:"message,omitempty"`
}

func home(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	response := httpbp.Response{
		Body: body{
			X: 1,
			Y: 2,
		},
	}
	return httpbp.WriteJSON(w, response)
}

func err(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	body := errorResponse{
		Reason:  "BAD_REQUEST",
		Message: "this endpoint always returns an error",
	}
	return httpbp.NewJSONError(http.StatusBadRequest, body, errors.New("example"))
}

func loggingMiddleware(name string, next httpbp.HandlerFunc) httpbp.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		log.Infof("Request %q: %#v", name, r)
		return next(ctx, w, r)
	}
}

var (
	_ httpbp.HandlerFunc = home
	_ httpbp.Middleware  = loggingMiddleware
)

func ExampleBaseplateHandlerFactory() {
	var (
		ecImpl       *edgecontext.Impl
		trustHandler httpbp.HeaderTrustHandler
	)
	handlerFactory := httpbp.BaseplateHandlerFactory{
		// arg struct for http.DefaultMiddleware
		Args: httpbp.DefaultMiddlewareArgs{
			TrustHandler:    trustHandler,
			EdgeContextImpl: ecImpl,
		},
		// Middleware that will be applied to each endpoint created by this factory
		Middlewares: []httpbp.Middleware{
			loggingMiddleware,
		},
	}
	handler := http.NewServeMux()
	handler.Handle("/", handlerFactory.NewHandler("home", home))
	handler.Handle("/err", handlerFactory.NewHandler("err", err))
	log.Fatal(http.ListenAndServe(":8080", handler))
}
