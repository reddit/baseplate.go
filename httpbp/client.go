package httpbp

import (
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/reddit/baseplate.go/tracing"
	"github.com/sony/gobreaker"
)

// LowLevelClientMiddleware is suitable for transport-level functions.  Retries
// and request munging should be done by wrapping the client.
type LowLevelClientMiddleware func(name string, next http.RoundTripper) http.RoundTripper

type maxConcurrency struct {
	next           http.RoundTripper
	activeRequests int64
	maxConcurrency int64
}

func (m *maxConcurrency) RoundTrip(req *http.Request) (*http.Response, error) {
	attemptedRequests := atomic.AddInt64(&m.activeRequests, 1)
	defer atomic.AddInt64(&m.activeRequests, -1)
	if m.maxConcurrency > 0 && attemptedRequests > m.maxConcurrency {
		return nil, errors.New("hit concurrency limit on http round-tripper")
	}
	return m.next.RoundTrip(req)
}

// MaxConcurrency bounds the total number of requests in-flight, erroring if the
// limit is exceeded.
func MaxConcurrency(max int64) LowLevelClientMiddleware {
	return func(name string, next http.RoundTripper) http.RoundTripper {
		return &maxConcurrency{
			next:           next,
			maxConcurrency: max,
		}
	}
}

type circuitBreaker struct {
	next    http.RoundTripper
	breaker *gobreaker.CircuitBreaker
}

func (c *circuitBreaker) RoundTrip(req *http.Request) (*http.Response, error) {
	rsp, err := c.breaker.Execute(func() (interface{}, error) {
		r, e := c.next.RoundTrip(req)
		if e != nil {
			return nil, e
		}
		if r.StatusCode > 499 {
			e = r.Body.Close()
			if e != nil {
				return nil, e
			}
			return nil, fmt.Errorf("received http %d", r.StatusCode)
		}
		return r, nil
	})
	if err != nil {
		return nil, err
	}
	return rsp.(*http.Response), nil
}

// CircuitBreaker enables circuit breaking for your client.
func CircuitBreaker(minRequests uint32, minFailureRatio float64) LowLevelClientMiddleware {
	settings := gobreaker.Settings{
		Interval: 60 * time.Second, // Reset the counts every 60 seconds
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= minRequests && failureRatio >= minFailureRatio
		},
	}
	breaker := gobreaker.NewCircuitBreaker(settings)

	return func(name string, next http.RoundTripper) http.RoundTripper {
		return &circuitBreaker{
			next:    next,
			breaker: breaker,
		}
	}
}

type spanWrapper struct {
	name string
	next http.RoundTripper
}

func (s *spanWrapper) RoundTrip(req *http.Request) (*http.Response, error) {
	_, ctx := opentracing.StartSpanFromContext(
		req.Context(),
		s.name,
		tracing.SpanTypeOption{Type: tracing.SpanTypeClient},
	)
	rsp, err := s.next.RoundTrip(req)
	if span := opentracing.SpanFromContext(ctx); span != nil {
		span.FinishWithOptions(tracing.FinishOptions{
			Ctx: req.Context(),
			Err: err,
		}.Convert())
	}
	return rsp, err
}

// SpanWrapper enables tracing for your client
func SpanWrapper(suffix string) LowLevelClientMiddleware {
	return func(name string, next http.RoundTripper) http.RoundTripper {
		return &spanWrapper{
			name: name + suffix,
			next: next,
		}
	}
}

// ClientConfig provides options for configuring a typical http client.
type ClientConfig struct {
	MaxConnsPerHost int
	Timeout         time.Duration
	Middlewares     []LowLevelClientMiddleware
}

// NewTypicalClientConfig contains the settings that feel reasonable
// under many conditions.  If you are calling out to a very
// high-performance and/or concurrent service, you may want to
// adjust values to maximize your performance. If you are calling
// out to a low-performance service, these values should be better
// than defaults, but you may still want to adjust in order to
// maximize safety.
func NewTypicalClientConfig() *ClientConfig {
	return &ClientConfig{
		MaxConnsPerHost: 20,
		Timeout:         1 * time.Second,
		Middlewares: []LowLevelClientMiddleware{
			CircuitBreaker(100, 0.1),
			MaxConcurrency(50),
			SpanWrapper(""),
		},
	}
}

// NewClient will create an HttpClient with the configured transport and
// transport middleware.  Note that retries are not suitable for low-level
// middleware, and you should consider bringing your own retries, wrapping
// this client.
func NewClient(config *ClientConfig) *http.Client {
	var tripper http.RoundTripper = &http.Transport{
		MaxConnsPerHost: config.MaxConnsPerHost,
	}
	for i := len(config.Middlewares) - 1; i >= 0; i-- {
		mw := config.Middlewares[i]
		tripper = mw(fmt.Sprintf("%v", mw), tripper)
	}

	return &http.Client{
		Timeout:   config.Timeout,
		Transport: tripper,
	}
}
