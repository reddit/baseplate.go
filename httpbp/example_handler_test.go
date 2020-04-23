package httpbp_test

import (
	"context"
	"errors"
	"net/http"
	"time"

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
	return httpbp.JSONError(httpbp.InternalServerError(), errors.New("example"))
}

func ratelimit(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	return httpbp.JSONError(
		httpbp.TooManyRequests().Retryable(w, time.Minute),
		errors.New("rate-limit"),
	)
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
	handler.Handle("/ratelimit", handlerFactory.NewHandler("ratelimit", ratelimit))
	log.Fatal(http.ListenAndServe(":8080", handler))
}
