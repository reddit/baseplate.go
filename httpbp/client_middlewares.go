package httpbp

import (
	"errors"
	"io"
	"net/http"
	"sync"
	"sync/atomic"

	retry "github.com/avast/retry-go"
	opentracing "github.com/opentracing/opentracing-go"

	"github.com/reddit/baseplate.go/breakerbp"
	"github.com/reddit/baseplate.go/retrybp"
	"github.com/reddit/baseplate.go/tracing"
)

const (
	// DefaultMaxErrorReadAhead defines the maximum bytes to be read from a
	// failed HTTP response to be attached as additional information in a
	// ClientError response.
	DefaultMaxErrorReadAhead = 1024
)

// ClientMiddleware is used to build HTTP client middleware by implementing
// http.RoundTripper which http.Client accepts as Transport.
type ClientMiddleware func(next http.RoundTripper) http.RoundTripper

// roundTripperFunc adapts closures and functions to implement http.RoundTripper.
type roundTripperFunc func(req *http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// NewClient returns a standard HTTP client wrapped with the default middleware
// plus any additional client middleware passed into this function. Default
// middlewares are: MonitorClient and Retries. ClientErrorWrapper is included
// as transitive middleware through Retries.
func NewClient(config ClientConfig, middleware ...ClientMiddleware) (*http.Client, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	// set max connections per host if set
	var transport http.Transport
	if config.MaxConnections > 0 {
		transport.MaxConnsPerHost = config.MaxConnections
	}

	// apply default if not set
	if config.MaxErrorReadAhead == 0 {
		config.MaxErrorReadAhead = DefaultMaxErrorReadAhead
	}

	// if no retry options are set default to retry.Attempts(1)
	if len(config.RetryOptions) == 0 {
		config.RetryOptions = []retry.Option{retry.Attempts(1)}
	}

	defaults := []ClientMiddleware{
		MonitorClient(config.Slug),
		Retries(config.MaxErrorReadAhead, config.RetryOptions...),
	}

	// prepend middleware to ensure Retires with ClientErrorWrapper is still
	// applied first
	if config.CircuitBreaker != nil {
		defaults = append([]ClientMiddleware{CircuitBreaker(*config.CircuitBreaker)}, defaults...)
	}
	middleware = append(middleware, defaults...)

	return &http.Client{
		Transport: WrapTransport(&transport, middleware...),
	}, nil
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
// ensuring an HTTP status response outside the range [200, 400) is wrapped in
// an error relieving users from the need to check the status response.
//
// If a response is wrapped in an error this middleware will perform
// DrainAndClose on the response body and will read up to limit to store
// ClientError.AdditionalInfo about the HTTP response.
//
// In the event of an error the response payload is read up to number of
// maxErrorReadAhead bytes. If the parameter is set to a value <= 0 it will be
// set to DefaultMaxErrorReadAhead.
func ClientErrorWrapper(maxErrorReadAhead int) ClientMiddleware {
	if maxErrorReadAhead <= 0 {
		maxErrorReadAhead = DefaultMaxErrorReadAhead
	}
	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(req *http.Request) (resp *http.Response, err error) {
			resp, err = next.RoundTrip(req)
			if err != nil {
				return nil, err
			}
			err = ClientErrorFromResponse(resp)
			if err != nil {
				defer DrainAndClose(resp.Body)
				var ce *ClientError
				if !errors.As(err, &ce) {
					return nil, err
				}
				body, e := io.ReadAll(io.LimitReader(resp.Body, int64(maxErrorReadAhead)))
				if e != nil {
					return nil, e
				}
				ce.AdditionalInfo = string(body)
				return nil, ce
			}
			return resp, nil
		})
	}
}

// CircuitBreaker is a middleware that prevents sending requests that are
// likely to fail through a configurable failure ratio based on total failures
// and requests. The circuit breaker is applied on a per-host basis, e.g.
// failed requests are counting per host.
func CircuitBreaker(config breakerbp.Config) ClientMiddleware {
	var breakers sync.Map
	pool := sync.Pool{
		New: func() interface{} {
			return breakerbp.NewFailureRatioBreaker(config)
		},
	}

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
			if resp != nil {
				return resp.(*http.Response), err
			}
			return nil, err
		})
	}
}

// Retries provides a retry middleware by ensuring certain HTTP responses are
// wrapped in errors. Retries wraps the ClientErrorWrapper middleware, e.g. if
// you are using Retries there is no need to also use ClientErrorWrapper.
func Retries(limit int, retryOptions ...retry.Option) ClientMiddleware {
	if len(retryOptions) == 0 {
		retryOptions = []retry.Option{retry.Attempts(1)}
	}
	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(req *http.Request) (resp *http.Response, err error) {
			err = retrybp.Do(req.Context(), func() error {
				// include ClientErrorWrapper to ensure retry is applied for
				// some HTTP 5xx responses
				resp, err = ClientErrorWrapper(limit)(next).RoundTrip(req)
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

// MonitorClient is an HTTP client middleware that wraps HTTP requests in a
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
