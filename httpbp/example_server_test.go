package httpbp_test

import (
	"context"
	"errors"
	"net/http"
	"time"

	baseplate "github.com/reddit/baseplate.go"
	"github.com/reddit/baseplate.go/ecinterface"
	"github.com/reddit/baseplate.go/httpbp"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/redis/deprecated/redisbp"
	"github.com/reddit/baseplate.go/secrets"
)

type config struct {
	baseplate.Config `yaml:",inline"`

	Redis redisbp.ClusterConfig `yaml:"redis"`
}

type body struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type Handlers struct {
	secrets    *secrets.Store
	redisAddrs []string
}

func (h Handlers) Home(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	return httpbp.WriteJSON(w, httpbp.NewResponse(body{X: 1, Y: 2}))
}

func (h Handlers) ServerErr(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	return httpbp.JSONError(httpbp.InternalServerError(), errors.New("example"))
}

func (h Handlers) Ratelimit(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	return httpbp.JSONError(
		httpbp.TooManyRequests().Retryable(w, time.Minute),
		errors.New("rate-limit"),
	)
}

func (h Handlers) InvalidInput(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	return httpbp.JSONError(
		httpbp.BadRequest().WithDetails(map[string]string{
			"foo": "must be >= 0",
			"bar": "must be non-nil",
		}),
		errors.New("invalid-input"),
	)
}

func (h Handlers) Endpoints() map[httpbp.Pattern]httpbp.Endpoint {
	return map[httpbp.Pattern]httpbp.Endpoint{
		"/": {
			Name:    "home",
			Handle:  h.Home,
			Methods: []string{http.MethodGet},
		},
		"/err": {
			Name:    "err",
			Handle:  h.ServerErr,
			Methods: []string{http.MethodGet, http.MethodPost},
		},
		"/ratelimit": {
			Name:    "ratelimit",
			Handle:  h.Ratelimit,
			Methods: []string{http.MethodGet},
		},
		"/invalid-input": {
			Name:    "invalid-input",
			Handle:  h.InvalidInput,
			Methods: []string{http.MethodPost},
		},
	}
}

func loggingMiddleware(name string, next httpbp.HandlerFunc) httpbp.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		log.Infof("Request %q: %#v", name, r)
		return next(ctx, w, r)
	}
}

var (
	_ httpbp.Middleware = loggingMiddleware
)

// This example demonstrates what a typical main function should look like for a
// Baseplate HTTP service.
func ExampleNewBaseplateServer() {
	var cfg config
	// In real code this MUST be replaced by the factory from the actual implementation.
	var ecFactory ecinterface.Factory
	ctx, bp, err := baseplate.New(context.Background(), baseplate.NewArgs{
		ConfigPath:         "example.yaml",
		EdgeContextFactory: ecFactory,
		ServiceCfg:         &cfg,
	})
	if err != nil {
		panic(err)
	}
	defer bp.Close()

	handlers := Handlers{
		secrets:    bp.Secrets(),
		redisAddrs: cfg.Redis.Addrs,
	}
	server, err := httpbp.NewBaseplateServer(httpbp.ServerArgs{
		Baseplate:   bp,
		Endpoints:   handlers.Endpoints(),
		Middlewares: []httpbp.Middleware{loggingMiddleware},
	})
	if err != nil {
		panic(err)
	}
	log.Info(baseplate.Serve(ctx, baseplate.ServeArgs{Server: server}))
}
