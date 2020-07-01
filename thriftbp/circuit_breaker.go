package thriftbp

import (
	"context"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/metricsbp"
	"github.com/sony/gobreaker"
)

// FailureRatioBreaker is a circuit breaker based on gobreaker that uses a low-water-mark and
// % failure threshold to trip.
type FailureRatioBreaker struct {
	*gobreaker.CircuitBreaker

	metricPrefix      string
	minRequestsToTrip int
	failureThreshold  float64
}

// CircuitBreakerConfig represents the configuration for a FailureRatioBreaker
type CircuitBreakerConfig struct {
	// Minimum requests that need to be sent during a time period before the breaker is eligible to transition from closed to open.
	MinRequestsToTrip int
	// Percentage of requests that need to fail during a time period for the breaker to transition from closed to open.
	// Represented as a float in {0,1}, where .05 means >5% failures will trip the breaker.
	FailureThreshold float64
	// Gobreaker settings used to configure the underlying circuit breaker.
	Settings gobreaker.Settings
}

// NewFailureRatioBreaker creates a new FailureRatioBreaker with the provided configuration.
func NewFailureRatioBreaker(config CircuitBreakerConfig) FailureRatioBreaker {

	failureBreaker := FailureRatioBreaker{
		metricPrefix:      config.Settings.Name,
		minRequestsToTrip: config.MinRequestsToTrip,
		failureThreshold:  config.FailureThreshold,
	}
	config.Settings.OnStateChange = stateChanged
	config.Settings.ReadyToTrip = failureBreaker.ShouldTripCircuitBreaker
	failureBreaker.CircuitBreaker = gobreaker.NewCircuitBreaker(config.Settings)
	go failureBreaker.runStatsProducer()
	return failureBreaker
}

func (cb FailureRatioBreaker) runStatsProducer() {
	circuitBreakerGauge := metricsbp.M.RuntimeGauge(cb.metricPrefix + ".circuit.breaker.closed")

	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-metricsbp.M.Ctx().Done():
			return
		case <-tick.C:
			if cb.State() == gobreaker.StateOpen {
				circuitBreakerGauge.Set(0)
			} else {
				circuitBreakerGauge.Set(1)
			}
		}
	}
}

// ThriftMiddleware is a thrift.ClientMiddleware that handles circuit breaking.
func (cb FailureRatioBreaker) ThriftMiddleware(next thrift.TClient) thrift.TClient {
	return thrift.WrappedTClient{
		Wrapped: func(ctx context.Context, method string, args, result thrift.TStruct) error {
			_, err := cb.Execute(func() (interface{}, error) {
				return nil, next.Call(ctx, method, args, result)
			})
			return err
		},
	}
}

// ShouldTripCircuitBreaker checks if the circuit breaker should be tripped, based on the provided breaker counts.
func (cb FailureRatioBreaker) ShouldTripCircuitBreaker(counts gobreaker.Counts) bool {
	if counts.Requests > 0 && counts.Requests >= uint32(cb.minRequestsToTrip) {
		failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
		if failureRatio >= cb.failureThreshold {
			log.Warnw(
				"tripping circuit breaker",
				"name", cb.Name(),
				"requests", counts.Requests,
				"total_successes", counts.TotalSuccesses,
				"total_failures", counts.TotalFailures,
				"consecutive_successes", counts.ConsecutiveSuccesses,
				"consecutive_failures", counts.ConsecutiveFailures,
			)
			return true
		}
	}
	return false
}

func stateChanged(name string, from gobreaker.State, to gobreaker.State) {
	log.Infow("circuit breaker state changed", "from", from, "to", to)
}
