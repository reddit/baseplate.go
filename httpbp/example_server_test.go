package httpbp_test

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/reddit/baseplate.go"
	"github.com/reddit/baseplate.go/admin"
	"github.com/reddit/baseplate.go/ecinterface"
	"github.com/reddit/baseplate.go/httpbp"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/redis/db/redisbp"
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

type isHealthyResponse struct {
	Status string `json:"status,omitempty"`
}

type TestService struct {
	secrets    *secrets.Store
	redisAddrs []string
}

func (s *TestService) IsHealthy(w http.ResponseWriter, r *http.Request) {
	httpbp.NewResponse(isHealthyResponse{Status: "healthy"})
}

func (s *TestService) Home(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	return httpbp.WriteJSON(w, httpbp.NewResponse(body{X: 1, Y: 2}))
}

func (s *TestService) ServerErr(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	return httpbp.JSONError(httpbp.InternalServerError(), errors.New("example"))
}

func (s *TestService) Ratelimit(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	return httpbp.JSONError(
		httpbp.TooManyRequests().Retryable(w, time.Minute),
		errors.New("rate-limit"),
	)
}

func (s *TestService) InvalidInput(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	return httpbp.JSONError(
		httpbp.BadRequest().WithDetails(map[string]string{
			"foo": "must be >= 0",
			"bar": "must be non-nil",
		}),
		errors.New("invalid-input"),
	)
}

func (s *TestService) Endpoints() map[httpbp.Pattern]httpbp.Endpoint {
	return map[httpbp.Pattern]httpbp.Endpoint{
		"/": {
			Name:    "home",
			Handle:  s.Home,
			Methods: []string{http.MethodGet},
		},
		"/err": {
			Name:    "err",
			Handle:  s.ServerErr,
			Methods: []string{http.MethodGet, http.MethodPost},
		},
		"/ratelimit": {
			Name:    "ratelimit",
			Handle:  s.Ratelimit,
			Methods: []string{http.MethodGet},
		},
		"/invalid-input": {
			Name:    "invalid-input",
			Handle:  s.InvalidInput,
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
	// In real code this MUST be replaced by the factory from the actual implementation.
	var ecFactory ecinterface.Factory

	var cfg config
	if err := baseplate.ParseConfigYAML(&cfg); err != nil {
		panic(err)
	}
	ctx, bp, err := baseplate.New(context.Background(), baseplate.NewArgs{
		Config:             cfg,
		EdgeContextFactory: ecFactory,
	})
	if err != nil {
		panic(err)
	}
	defer bp.Close()

	svc := TestService{
		secrets:    bp.Secrets(),
		redisAddrs: cfg.Redis.Addrs,
	}
	server, err := httpbp.NewBaseplateServer(httpbp.ServerArgs{
		Baseplate:   bp,
		Endpoints:   svc.Endpoints(),
		Middlewares: []httpbp.Middleware{loggingMiddleware},
	})
	if err != nil {
		panic(err)
	}
	adminServer := admin.NewServer(&admin.ServerArgs{HealthCheckFn: svc.IsHealthy})
	go adminServer.Serve()
	log.Info(baseplate.Serve(ctx, baseplate.ServeArgs{Server: server}))
}
