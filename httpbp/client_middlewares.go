package httpbp

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/avast/retry-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/reddit/baseplate.go/headerbp"
	"github.com/reddit/baseplate.go/secrets"

	"github.com/reddit/baseplate.go/breakerbp"
	//lint:ignore SA1019 This library is internal only, not actually deprecated
	"github.com/reddit/baseplate.go/internalv2compat"
	"github.com/reddit/baseplate.go/retrybp"
	"github.com/reddit/baseplate.go/transport"
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
// middlewares are:
//
// * MonitorClient with transport.WithRetrySlugSuffix
//
// * PrometheusClientMetrics with transport.WithRetrySlugSuffix
//
// * Retries
//
// * MonitorClient
//
// * PrometheusClientMetrics
//
// * clientFaultMiddleware
//
// ClientErrorWrapper is included as transitive middleware through Retries.
//
// IMPORTANT: clientFaultMiddleware MUST be the last middleware as it simulates
// faults as if they originated from the upstream server.
func NewClient(config ClientConfig, middleware ...ClientMiddleware) (*http.Client, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	// set max connections per host if set
	var httpTransport http.Transport
	if config.MaxConnections > 0 {
		httpTransport.MaxConnsPerHost = config.MaxConnections
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
		MonitorClient(config.Slug + transport.WithRetrySlugSuffix),
		PrometheusClientMetrics(config.Slug + transport.WithRetrySlugSuffix),
		Retries(config.MaxErrorReadAhead, config.RetryOptions...),
		MonitorClient(config.Slug),
		PrometheusClientMetrics(config.Slug),
	}

	// prepend middleware to ensure Retires with ClientErrorWrapper is still
	// applied first
	if config.CircuitBreaker != nil {
		defaults = append([]ClientMiddleware{CircuitBreaker(*config.CircuitBreaker)}, defaults...)
	}

	// only add the middleware to forward baseplate headers if the client is configured for it
	if config.SecretsStore != nil && config.HeaderbpSigningKeyPath != "" {
		defaults = append(defaults, ClientBaseplateHeadersMiddleware(config.Slug, config.SecretsStore, config.HeaderbpSigningKeyPath))
	}

	middleware = append(middleware, defaults...)

	// ensure client fault middleware is applied last
	clientFaultMiddleware := NewClientFaultMiddleware(config.Slug)
	middleware = append(middleware, clientFaultMiddleware.Middleware())

	return &http.Client{
		Transport: WrapTransport(&httpTransport, middleware...),
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
			breaker := breakerbp.NewFailureRatioBreaker(config)
			return &breaker
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
				pool.Put(newBreaker)
				newBreaker = nil
			}

			var resp *http.Response
			_, err := breaker.(*breakerbp.FailureRatioBreaker).Execute(func() (interface{}, error) {
				r, err := next.RoundTrip(req)
				if err != nil {
					return nil, err
				}
				resp = r
				// circuit break on any HTTP 5xx code
				if resp.StatusCode >= http.StatusInternalServerError {
					DrainAndClose(resp.Body)
					return nil, ClientError{
						Status:     resp.Status,
						StatusCode: resp.StatusCode,
					}
				}
				return nil, nil
			})
			return resp, err
		})
	}
}

// Retries provides a retry middleware by ensuring certain HTTP responses are
// wrapped in errors. Retries wraps the ClientErrorWrapper middleware, e.g. if
// you are using Retries there is no need to also use ClientErrorWrapper.
func Retries(maxErrorReadAhead int, retryOptions ...retry.Option) ClientMiddleware {
	if len(retryOptions) == 0 {
		retryOptions = []retry.Option{retry.Attempts(1)}
	}
	return func(next http.RoundTripper) http.RoundTripper {
		// include ClientErrorWrapper to ensure retry is applied for some HTTP 5xx
		// responses
		next = ClientErrorWrapper(maxErrorReadAhead)(next)

		return roundTripperFunc(func(req *http.Request) (resp *http.Response, err error) {
			if req.Body != nil && req.Body != http.NoBody && req.GetBody == nil {
				slog.WarnContext(
					req.Context(),
					"Request comes with a Body but nil GetBody cannot be retried. httpbp.Retries middleware skipped.",
					"req", req,
				)
				return next.RoundTrip(req)
			}

			err = retrybp.Do(req.Context(), func() error {
				req = req.Clone(req.Context())
				if req.GetBody != nil {
					body, err := req.GetBody()
					if err != nil {
						return fmt.Errorf("httpbp.Retries: GetBody returned error: %w", err)
					}
					req.Body = body
				}

				resp, err = next.RoundTrip(req)
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
		activeRequests atomic.Int64
	)
	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(req *http.Request) (resp *http.Response, err error) {
			attemptedRequests := activeRequests.Add(1)
			defer activeRequests.Add(-1)

			if maxConcurrency > 0 && attemptedRequests > maxConcurrency {
				return nil, ErrConcurrencyLimit
			}
			return next.RoundTrip(req)
		})
	}
}

var monitorClientLoggingOnce sync.Once

// MonitorClient is an HTTP client middleware that wraps HTTP requests in a
// client span.
//
// This middleware always use the injected v2 tracing http client middleware.
// If there's no v2 tracing http client middleware injected, it's no-op.
func MonitorClient(slug string) ClientMiddleware {
	if mw := internalv2compat.V2TracingHTTPClientMiddleware(); mw != nil {
		return mw
	}
	return func(next http.RoundTripper) http.RoundTripper {
		// no-op but log for once
		monitorClientLoggingOnce.Do(func() {
			slog.Warn("httpbp.MonitorClient: internalv2compat.V2TracingHTTPClientMiddleware() returned nil")
		})
		return next
	}
}

// PrometheusClientMetrics returns a middleware that tracks Prometheus metrics for client http.
//
// It emits the following prometheus metrics:
//
// * http_client_active_requests gauge with labels:
//
//   - http_method: method of the HTTP request
//   - http_client_name: the remote service being contacted, the serverSlug arg
//
// * http_client_latency_seconds histogram with labels above plus:
//
//   - http_success: true if the status code is 2xx or 3xx, false otherwise
//
// * http_client_requests_total counter with all labels above plus:
//
//   - http_response_code: numeric status code as a string, e.g. 200
func PrometheusClientMetrics(serverSlug string) ClientMiddleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(req *http.Request) (resp *http.Response, err error) {
			start := time.Now()
			method := req.Method
			activeRequestLabels := prometheus.Labels{
				methodLabel:     method,
				clientNameLabel: serverSlug,
				endpointLabel:   "",
			}
			clientActiveRequests.With(activeRequestLabels).Inc()

			defer func() {
				// the Retries middleware might return nil resp with an error,
				// in such case, try to get it from ClientError instead,
				// but fallback to a 5xx error code if nothing is available.
				code := 599
				var ce *ClientError
				if errors.As(err, &ce) {
					code = ce.StatusCode
				}
				if resp != nil {
					code = resp.StatusCode
				}
				success := isRequestSuccessful(code, err)

				latencyLabels := prometheus.Labels{
					methodLabel:     method,
					successLabel:    success,
					clientNameLabel: serverSlug,
					endpointLabel:   "",
				}

				clientLatencyDistribution.With(latencyLabels).Observe(time.Since(start).Seconds())

				totalRequestLabels := prometheus.Labels{
					methodLabel:     method,
					successLabel:    success,
					codeLabel:       strconv.Itoa(code),
					clientNameLabel: serverSlug,
					endpointLabel:   "",
				}

				clientTotalRequests.With(totalRequestLabels).Inc()
				clientActiveRequests.With(activeRequestLabels).Dec()
			}()

			return next.RoundTrip(req)
		})
	}
}

// ClientBaseplateHeadersMiddleware is a middleware that forwards baseplate headers from the context to the outgoing request.
//
// If it detects any new baseplate headers set on the request, it will reject the request and return an error.
func ClientBaseplateHeadersMiddleware(client string, store SecretsStore, path string) ClientMiddleware {
	getSigningSecret := func() *secrets.VersionedSecret {
		secret, err := store.GetVersionedSecret(path)
		if err != nil {
			slog.Error(
				"Failed to get secret",
				"path", path,
				"err", err,
			)
		}
		return &secret
	}

	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			ctx := req.Context()

			if headerbp.HasSetOutgoingHeaders(ctx, headerbp.WithHTTPClient("", client, "")) {
				return next.RoundTrip(req)
			}

			signingSecret := getSigningSecret()
			if signingSecret == nil {
				return nil, fmt.Errorf("signing secret is required to set use baseplate headers")
			}

			for k := range req.Header {
				if headerbp.ShouldRemoveClientHeader(k,
					headerbp.WithHTTPClient("", client, ""),
				) {
					req.Header.Del(k)
				}
			}

			signature, hasSignature := headerbp.HeaderSignatureFromContext(ctx)
			var baseplateHeaders []string
			ctx = headerbp.SetOutgoingHeaders(
				ctx,
				headerbp.WithHTTPClient("", client, ""),
				headerbp.WithHeaderSetter(func(key, value string) {
					if !hasSignature {
						baseplateHeaders = append(baseplateHeaders, key)
					}
					req.Header.Set(key, value)
				}),
			)
			// the !hasSignature check is redundant since we do not add to the baseplateHeaders list unless it is false, but
			// it's here to make it clear that we only need to update the signature if there is no signature in the context
			if len(baseplateHeaders) > 0 && !hasSignature {
				if _signature, err := headerbp.SignHeaders(ctx, *signingSecret, baseplateHeaders, req.Header.Get); err != nil {
					return nil, fmt.Errorf("signing baseplate headers: %w", err)
				} else {
					signature = _signature
				}
			}
			if signature != "" {
				req.Header.Set(headerbp.SignatureHeaderCanonicalHTTP, signature)
			}
			req = req.WithContext(ctx)
			return next.RoundTrip(req)
		})
	}
}

// Middleware returns a middleware that injects faults into the outgoing HTTP
// requests based on the X-Bp-Fault header values.
func (c clientFaultMiddleware) Middleware() ClientMiddleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			resume := func() (*http.Response, error) {
				return next.RoundTrip(req)
			}

			abort := func(code int, message string) (*http.Response, error) {
				return &http.Response{
					Status:     http.StatusText(code),
					StatusCode: code,
					Proto:      req.Proto,
					ProtoMajor: req.ProtoMajor,
					ProtoMinor: req.ProtoMinor,
					Header: map[string][]string{
						// Copied from the standard http.Error() function.
						"Content-Type":           {"text/plain; charset=utf-8"},
						"X-Content-Type-Options": {"nosniff"},
					},
					Body:             http.NoBody,
					ContentLength:    0,
					TransferEncoding: req.TransferEncoding,
					Request:          req,
					TLS:              req.TLS,
				}, nil
			}

			address := req.URL.Hostname()
			method := strings.TrimPrefix(req.URL.Path, "/")
			return c.injector.InjectWithAbortOverride(req.Context(), address, method, httpHeaders{req}, resume, abort)
		})
	}
}
