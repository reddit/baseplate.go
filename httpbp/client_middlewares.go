package httpbp

import (
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	retry "github.com/avast/retry-go"
	"github.com/opentracing/opentracing-go"

	"github.com/reddit/baseplate.go/retrybp"
	"github.com/reddit/baseplate.go/tracing"
	"github.com/sony/gobreaker"
)

// ClientMiddleware is used to build HTTP client middlewares by implementing
// http.RoundTripper which http.Client accepts as Transport.
type ClientMiddleware func(roundTripper http.RoundTripper) http.RoundTripper

type roundTripper func(req *http.Request) (*http.Response, error)

func (f roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// NewClient returns a standard HTTP client wrapped with the default
// middlewares plus any additional client middlewares passed into this
// function.
func NewClient(config Config, middlewares ...ClientMiddleware) *http.Client {
	transport := &http.Transport{
		MaxConnsPerHost: config.MaxConnections,
	}

	defaults := []ClientMiddleware{
		CircuitBreaker(config),
	}

	middlewares = append(middlewares, defaults...)

	return &http.Client{
		Transport: applyMiddlewares(transport, middlewares...),
	}
}

// CircuitBreaker is a middleware that prevents sending requests that are likely
// to fail based on a configurable failure ratio based on total failures and
// requests.
func CircuitBreaker(config Config) ClientMiddleware {
	pool := &sync.Pool{
		New: func() interface{} {
			return &gobreaker.Settings{
				Interval: 60 * time.Second,
				ReadyToTrip: func(counts gobreaker.Counts) bool {
					failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
					return counts.Requests >= config.CircuitBreaker.MinRequests && failureRatio >= config.CircuitBreaker.MinFailureRatio
				},
			}
		},
	}
	breakers := &sync.Map{}

	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripper(func(req *http.Request) (*http.Response, error) {
			host := req.URL.Hostname()

			// new circuit breakers should rarely get allocated: the pool reduces
			// garbage collector overhead
			newBreaker := pool.Get()
			breaker, loaded := breakers.LoadOrStore(host, newBreaker)
			if loaded {
				pool.Put(newBreaker)
			}

			resp, err := breaker.(*gobreaker.CircuitBreaker).Execute(func() (interface{}, error) {
				resp, err := next.RoundTrip(req)
				if err != nil {
					return nil, err
				}
				// circuit break on any HTTP 5xx code
				if resp.StatusCode >= http.StatusInternalServerError {
					err = resp.Body.Close()
					if err != nil {
						return nil, err
					}
					return nil, err
				}
				return resp, nil
			})
			if err != nil {
				return nil, err
			}
			return resp.(*http.Response), nil
		})
	}
}

func applyMiddlewares(transport http.RoundTripper, middlewares ...ClientMiddleware) http.RoundTripper {
	// add middlewares in reverse so the first in the list is the outermost
	for i := len(middlewares) - 1; i >= 0; i-- {
		transport = middlewares[i](transport)
	}
	return transport
}

// Retries provides a retry middleware.
func Retries(config retry.Config) ClientMiddleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripper(func(req *http.Request) (resp *http.Response, err error) {
			retrybp.Do(req.Context(), func() error {
				resp, err = next.RoundTrip(req)
				if err != nil {
					return err
				}
				return nil
			}, retry.Attempts(1))
			if err != nil {
				return nil, err
			}
			return resp, nil
		})
	}
}

// LoadBalancer implements a round-robin load balancer.
func LoadBalancer(config Config) ClientMiddleware {
	attempts := 0
	offset := uint64(0)
	hosts := config.LoadBalancer.Hosts
	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripper(func(req *http.Request) (resp *http.Response, err error) {

			remaining := attempts
			for remaining > 0 {
				i := atomic.AddUint64(&offset, 1)
				hostIndex := i % uint64(len(hosts))
				req.Host = hosts[hostIndex]
				req.URL.Host = hosts[hostIndex]

				resp, err := next.RoundTrip(req)
				if err == gobreaker.ErrOpenState || err == gobreaker.ErrTooManyRequests {
					remaining--
				} else {
					return resp, err
				}
			}
			return next.RoundTrip(req)
		})
	}
}

// MaxConcurrency is a middleware to ensure that there will be only a maximum
// number of requests concurrently in-flight at any given time.
func MaxConcurrency(config Config) ClientMiddleware {
	maxConcurrency := int64(config.MaxConcurrency)
	activeRequests := int64(0)
	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripper(func(req *http.Request) (resp *http.Response, err error) {
			attemptedRequests := atomic.AddInt64(&activeRequests, 1)
			defer atomic.AddInt64(&activeRequests, -1)

			if maxConcurrency > 0 && attemptedRequests > maxConcurrency {
				return nil, ErrConcurrencyLimit
			}
			return next.RoundTrip(req)
		})
	}
}

// MonitorClient is a HTTP client middleware that wraps HTTP requests in a
// client span.
func MonitorClient(config Config) ClientMiddleware {
	slug := config.Slug
	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripper(func(req *http.Request) (resp *http.Response, err error) {
			span, ctx := opentracing.StartSpanFromContext(
				req.Context(),
				slug,
				tracing.SpanTypeOption{Type: tracing.SpanTypeClient},
			)
			span.SetTag("http.method", req.Method)
			span.SetTag("http.url", req.URL)

			defer func() {
				span.FinishWithOptions(tracing.FinishOptions{
					Ctx: req.Context(),
					Err: err,
				}.Convert())
			}()
			req = req.WithContext(ctx)
			return next.RoundTrip(req)
		})
	}
}
