package httpbp

import (
	"errors"
	"io"
	"net/http"
	"sync"
	"sync/atomic"

	retry "github.com/avast/retry-go"
	"github.com/opentracing/opentracing-go"

	"github.com/reddit/baseplate.go/breakerbp"
	"github.com/reddit/baseplate.go/retrybp"
	"github.com/reddit/baseplate.go/tracing"
)

// ClientMiddleware is used to build HTTP client middleware by implementing
// http.RoundTripper which http.Client accepts as Transport.
type ClientMiddleware func(next http.RoundTripper) http.RoundTripper

// roundTripperFunc adapts closures and functions to implement http.RoundTripper.
type roundTripperFunc func(req *http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// NewClient returns a standard HTTP client wrapped with the default
// middleware plus any additional client middleware passed into this
// function.
func NewClient(config ClientConfig, middleware ...ClientMiddleware) *http.Client {
	transport := &http.Transport{
		MaxConnsPerHost: config.MaxConnections,
	}

	defaults := []ClientMiddleware{
		ClientErrorWrapper(),
		MonitorClient(config.Slug),
		Retries(
			retry.Attempts(1),
			retrybp.Filters(
				retrybp.NetworkErrorFilter,
				retrybp.RetryableErrorFilter,
			),
		),
	}
	if config.CircuitBreaker != nil {
		defaults = append(defaults, CircuitBreaker(*config.CircuitBreaker))
	}

	middleware = append(middleware, defaults...)

	return &http.Client{
		Transport: WrapTransport(transport, middleware...),
	}
}

// WrapTransport takes a list of client middleware and wraps them around the
// given transport. This is useful for using client middleware outside of this
// package.
func WrapTransport(transport http.RoundTripper, middleware ...ClientMiddleware) http.RoundTripper {
	if transport == nil {
		transport = http.DefaultTransport
	}
	// add middleware in reverse so the first in the list is the outermost
	for i := len(middleware) - 1; i >= 0; i-- {
		transport = middleware[i](transport)
	}
	return transport
}

// ClientErrorWrapper applies ClientErrorFromResponse to the returned response
// ensuring a HTTP status response outside the range [200, 400) is wrapped in
// an error relieving users from the need to check the status response.
//
// Adding this middleware means that both, non-nil response and non-nil error,
// can be returned. If configured the caller needs to call
// httpbp.DrainAndClose(resp.Body) to ensure the underlying TCP connection can
// be re-used.
func ClientErrorWrapper() ClientMiddleware {
	const limit = 1024
	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(req *http.Request) (resp *http.Response, err error) {
			resp, err = next.RoundTrip(req)
			if err != nil {
				return resp, err
			}
			if err == nil {
				err = ClientErrorFromResponse(resp)
				if err != nil {
					defer DrainAndClose(resp.Body)
					var ce *ClientError
					if !errors.As(err, &ce) {
						return resp, err
					}
					body, e := io.ReadAll(io.LimitReader(resp.Body, limit))
					if e != nil {
						return resp, e
					}
					ce.AdditionalInfo = string(body)
					return resp, ce
				}
			}
			return resp, err
		})
	}
}

// CircuitBreaker is a middleware that prevents sending requests that are likely
// to fail based on a configurable failure ratio based on total failures and
// requests.
func CircuitBreaker(config breakerbp.Config) ClientMiddleware {
	pool := &sync.Pool{
		New: func() interface{} {
			return breakerbp.NewFailureRatioBreaker(config)
		},
	}
	breakers := &sync.Map{}

	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			host := req.URL.Hostname()

			// new circuit breakers should rarely get allocated: the pool reduces
			// garbage collector overhead
			newBreaker := pool.Get()
			breaker, loaded := breakers.LoadOrStore(host, newBreaker)
			if loaded {
				defer pool.Put(newBreaker)
			}

			resp, err := breaker.(breakerbp.FailureRatioBreaker).Execute(func() (interface{}, error) {
				resp, err := next.RoundTrip(req)
				if err != nil {
					return nil, err
				}
				// circuit break on any HTTP 5xx code
				if resp.StatusCode >= http.StatusInternalServerError {
					// read & close to ensure underlying RoundTripper
					// (http.Transport) is able to re-use the persistent TCP
					// connection.
					_, _ = io.ReadAll(resp.Body)
					return nil, resp.Body.Close()
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

// Retries provides a retry middleware.
func Retries(retryOptions ...retry.Option) ClientMiddleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(req *http.Request) (resp *http.Response, err error) {
			if len(retryOptions) == 0 {
				retryOptions = []retry.Option{retry.Attempts(1)}
			}

			err = retrybp.Do(req.Context(), func() error {
				// include ClientErrorWrapper to ensure retry is applied for
				// some HTTP 5xx responses
				resp, err = ClientErrorWrapper()(next).RoundTrip(req)
				if err != nil {
					return err
				}
				return nil
			}, retryOptions...)
			if err != nil {
				return nil, err
			}
			return resp, nil
		})
	}
}

// MaxConcurrency is a middleware to limit the number of concurrent in-flight
// requests at any given time by returning an error if the maximum is reached.
func MaxConcurrency(maxConcurrency int64) ClientMiddleware {
	var (
		activeRequests int64
	)
	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(req *http.Request) (resp *http.Response, err error) {
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
func MonitorClient(slug string) ClientMiddleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(req *http.Request) (resp *http.Response, err error) {
			span, ctx := opentracing.StartSpanFromContext(
				req.Context(),
				slug+".request",
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
			return next.RoundTrip(req.WithContext(ctx))
		})
	}
}
