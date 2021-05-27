package breakerbp

import (
	"context"
	"fmt"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/sony/gobreaker"

	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/metricsbp"
)

// FailureRatioBreaker is a circuit breaker based on gobreaker that uses a low-water-mark and
// % failure threshold to trip.
type FailureRatioBreaker struct {
	goBreaker *gobreaker.CircuitBreaker

	name              string
	minRequestsToTrip int
	failureThreshold  float64
	logger            log.Wrapper
}

// Config represents the configuration for a FailureRatioBreaker.
type Config struct {
	// Minimum requests that need to be sent during a time period before the breaker is eligible to transition from closed to open.
	MinRequestsToTrip int `yaml:"minRequestsToTrip"`

	// Percentage of requests that need to fail during a time period for the breaker to transition from closed to open.
	// Represented as a float in [0,1], where .05 means >=5% failures will trip the breaker.
	FailureThreshold float64 `yaml:"failureThreshold"`

	// Name for this circuit breaker, mostly used as a prefix to disambiguate logs when multiple cb are used.
	Name string `yaml:"name"`

	// EmitStatusMetrics sets whether the failure breaker will regularly update a gauge on the breakers state (closed or open/halfopen).
	// When enabled, it emits metrics using the interval defined by metricsbp.SysStatsTickerInterval.
	EmitStatusMetrics bool `yaml:"emitStatusMetrics"`

	// Logger is the logger to be called when the breaker changes states.
	Logger log.Wrapper `yaml:"logger"`

	// MaxRequestsHalfOpen represents he Maximum amount of requests that will be allowed through while the breaker
	// is in half-open state. If left unset (or set to 0), exactly 1 request will be allowed through while half-open.
	MaxRequestsHalfOpen uint32 `yaml:"maxRequestsHalfOpen"`

	// Interval represents the cyclical period of the 'Closed' state.
	// If 0, internal counts do not get reset while the breaker remains in the Closed state.
	Interval time.Duration `yaml:"interval"`

	// Timeout is the duration of the 'Open' state. After an 'Open' timeout duration has passed, the breaker enters 'half-open' state.
	Timeout time.Duration `yaml:"timeout"`
}

// NewFailureRatioBreaker creates a new FailureRatioBreaker with the provided configuration. Creates a new goroutine to emit
// breaker state metrics if EmitStatusMetrics is set to true. This goroutine is stopped when metricsbp.M.Ctx() is done().
func NewFailureRatioBreaker(config Config) FailureRatioBreaker {

	failureBreaker := FailureRatioBreaker{
		name:              config.Name,
		minRequestsToTrip: config.MinRequestsToTrip,
		failureThreshold:  config.FailureThreshold,
		logger:            config.Logger,
	}
	settings := gobreaker.Settings{
		Name:          config.Name,
		Interval:      config.Interval,
		Timeout:       config.Timeout,
		MaxRequests:   config.MaxRequestsHalfOpen,
		ReadyToTrip:   failureBreaker.shouldTrip,
		OnStateChange: failureBreaker.stateChanged,
	}

	failureBreaker.goBreaker = gobreaker.NewCircuitBreaker(settings)
	if config.EmitStatusMetrics {
		go failureBreaker.runStatsProducer()
	}
	return failureBreaker
}

func (cb FailureRatioBreaker) runStatsProducer() {
	circuitBreakerGauge := metricsbp.M.RuntimeGauge(cb.name + "-circuit-breaker-closed")

	tick := time.NewTicker(metricsbp.SysStatsTickerInterval)
	defer tick.Stop()
	for {
		select {
		case <-metricsbp.M.Ctx().Done():
			return
		case <-tick.C:
			if cb.goBreaker.State() == gobreaker.StateOpen {
				circuitBreakerGauge.Set(0)
			} else {
				circuitBreakerGauge.Set(1)
			}
		}
	}
}

// Execute wraps the given function call in circuit breaker logic and returns
// the result.
func (cb FailureRatioBreaker) Execute(fn func() (interface{}, error)) (interface{}, error) {
	return cb.goBreaker.Execute(fn)
}

// ThriftMiddleware is a thrift.ClientMiddleware that handles circuit breaking.
func (cb FailureRatioBreaker) ThriftMiddleware(next thrift.TClient) thrift.TClient {
	return thrift.WrappedTClient{
		Wrapped: func(ctx context.Context, method string, args, result thrift.TStruct) (thrift.ResponseMeta, error) {
			m, err := cb.Execute(func() (interface{}, error) {
				return next.Call(ctx, method, args, result)
			})
			meta, _ := m.(thrift.ResponseMeta)
			return meta, err
		},
	}
}

// ShouldTrip checks if the circuit breaker should be tripped, based on the provided breaker counts.
func (cb FailureRatioBreaker) shouldTrip(counts gobreaker.Counts) bool {
	if counts.Requests > 0 && counts.Requests >= uint32(cb.minRequestsToTrip) {
		failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
		if failureRatio >= cb.failureThreshold {
			message := fmt.Sprintf("tripping circuit breaker: name=%v, counts=%v", cb.name, counts)
			cb.logger.Log(context.Background(), message)
			return true
		}
	}
	return false
}

func (cb FailureRatioBreaker) stateChanged(name string, from gobreaker.State, to gobreaker.State) {
	message := fmt.Sprintf("circuit breaker %v state changed from %v to %v", name, from, to)
	cb.logger.Log(context.Background(), message)
}

var (
	_ CircuitBreaker = FailureRatioBreaker{}
	_ CircuitBreaker = (*gobreaker.CircuitBreaker)(nil)
)
