package httpbp

import (
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/reddit/baseplate.go/breakerbp"
	"github.com/reddit/baseplate.go/tracing"
	"github.com/sony/gobreaker"
)

// ClientMiddleware is used to build HTTP client middlewares by implementing
// http.RoundTripper which http.Client accepts as Transport.
type ClientMiddleware func(roundTripper http.RoundTripper) http.RoundTripper

// NewClient returns a standard HTTP client wrapped with the default
// middlewares plus any additional client middlewares passed into this
// function.
func NewClient(config Config, middlewares ...ClientMiddleware) *http.Client {
	transport := &http.Transport{
		MaxConnsPerHost: config.MaxConnections,
	}
	defaults := []ClientMiddleware{
		MonitorClient(config),
		CircuitBreaker(config.CircuitBreaker),
		Retries(config),
	}
	middlewares = append(middlewares, defaults...)

	return &http.Client{
		Transport: applyMiddlewares(transport, middlewares...),
	}
}

func applyMiddlewares(transport http.RoundTripper, middlewares ...ClientMiddleware) http.RoundTripper {
	// add middlewares in reverse so the first in the list is the outermost
	for i := len(middlewares) - 1; i >= 0; i-- {
		transport = middlewares[i](transport)
	}
	return transport
}

type circuitBreaker struct {
	next     http.RoundTripper
	settings gobreaker.Settings

	mu       sync.RWMutex
	breakers map[string]breakerbp.CircuitBreaker
}

// CircuitBreaker is a middleware that prevents sending requests that are likely
// to fail based on a configurable failure ratio based on total failures and
// requests.
func CircuitBreaker(config CircuitBreakerConfig) ClientMiddleware {
	settings := gobreaker.Settings{
		Interval: 60 * time.Second, // Reset the counts every 60 seconds
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= config.MinRequests && failureRatio >= config.MinFailureRatio
		},
	}
	return func(next http.RoundTripper) http.RoundTripper {
		return &circuitBreaker{
			next:     next,
			settings: settings,
		}
	}
}

func (c *circuitBreaker) RoundTrip(req *http.Request) (*http.Response, error) {
	host := req.URL.Hostname()

	c.mu.RLock()
	breaker, ok := c.breakers[host]
	if !ok {
		c.setupBreaker(host)
	}
	c.mu.RUnlock()

	resp, err := breaker.Execute(c.roundTrip(req))
	if err != nil {
		return nil, err
	}
	return resp.(*http.Response), nil
}

func (c *circuitBreaker) setupBreaker(host string) {
	// this code path is executed rarely and usually at the start of a process
	c.mu.RUnlock()
	c.mu.Lock()
	c.breakers[host] = gobreaker.NewCircuitBreaker(c.settings)
	c.mu.Unlock()
	c.mu.RLock()
}

func (c *circuitBreaker) roundTrip(req *http.Request) func() (interface{}, error) {
	return func() (interface{}, error) {
		resp, err := c.next.RoundTrip(req)
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
	}
}

type retries struct {
	next    http.RoundTripper
	retries int
}

// Retries provides a retry middleware.
func Retries(config Config) ClientMiddleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return &retries{
			next:    next,
			retries: config.Retries,
		}
	}
}

func (r *retries) RoundTrip(req *http.Request) (*http.Response, error) {
	remaining := r.retries + 1
	for remaining > 0 {
		resp, err := r.next.RoundTrip(req)
		if err != nil {
			return nil, err
		}
		return resp, nil
	}
	return r.next.RoundTrip(req)
}

type loadBalancer struct {
	next     http.RoundTripper
	hosts    []string
	offset   uint64
	attempts int
}

// LoadBalancer implements a round-robin load balancer.
func LoadBalancer(config LoadBalancerConfig) ClientMiddleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return &loadBalancer{
			next:  next,
			hosts: config.Hosts,
		}
	}
}

func (lb *loadBalancer) RoundTrip(req *http.Request) (*http.Response, error) {
	remaining := lb.attempts
	for remaining > 0 {
		i := atomic.AddUint64(&lb.offset, 1)
		hostIndex := i % uint64(len(lb.hosts))
		req.Host = lb.hosts[hostIndex]
		req.URL.Host = lb.hosts[hostIndex]

		resp, err := lb.next.RoundTrip(req)
		if err == gobreaker.ErrOpenState || err == gobreaker.ErrTooManyRequests {
			remaining--
		} else {
			return resp, err
		}
	}
	return lb.next.RoundTrip(req)
}

type maxConcurrency struct {
	next           http.RoundTripper
	maxConcurrency int64
	activeRequests int64
}

// MaxConcurrency is a middleware to ensure that there will be only a maximum
// number of requests concurrently in-flight at any given time.
func MaxConcurrency(config Config) ClientMiddleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return &maxConcurrency{
			next:           next,
			maxConcurrency: int64(config.MaxConcurrency),
		}
	}
}

func (m *maxConcurrency) RoundTrip(req *http.Request) (*http.Response, error) {
	attemptedRequests := atomic.AddInt64(&m.activeRequests, 1)
	defer atomic.AddInt64(&m.activeRequests, -1)

	if m.maxConcurrency > 0 && attemptedRequests > m.maxConcurrency {
		return nil, ErrConcurrencyLimit
	}
	return m.next.RoundTrip(req)
}

type monitorClient struct {
	next http.RoundTripper
	slug string
}

// MonitorClient is a HTTP client middleware that wraps HTTP requests in a
// client span.
func MonitorClient(config Config) ClientMiddleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return &monitorClient{
			next: next,
			slug: config.Slug,
		}
	}
}

func (m *monitorClient) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	span, ctx := opentracing.StartSpanFromContext(
		req.Context(),
		m.slug,
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
	return m.next.RoundTrip(req)
}
