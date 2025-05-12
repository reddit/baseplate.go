package breakerbp

import (
	"context"
	"log/slog"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sony/gobreaker"

	"github.com/reddit/baseplate.go/internal/prometheusbpint"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/randbp"
)

const (
	nameLabel = "breaker"
)

var (
	breakerLabels = []string{
		nameLabel,
	}

	breakerClosed = promauto.With(prometheusbpint.GlobalRegistry).NewGaugeVec(prometheus.GaugeOpts{
		Name: "breakerbp_closed",
		Help: "0 means the breaker is currently tripped, 1 otherwise (closed)",
	}, breakerLabels)

	breakerTimeout = promauto.With(prometheusbpint.GlobalRegistry).NewGaugeVec(prometheus.GaugeOpts{
		Name: "breakerbp_jittered_timeout_seconds",
		Help: "The jittered timeout used by this breaker",
	}, breakerLabels)
)

// FailureRatioBreaker is a circuit breaker based on gobreaker that uses a low-water-mark and
// % failure threshold to trip.
type FailureRatioBreaker struct {
	goBreaker *gobreaker.CircuitBreaker

	name              string
	minRequestsToTrip int
	failureThreshold  float64
	logContext        context.Context
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
	// When enabled, it emits metrics using the interval of EmitStatusMetricsInterval.
	// If EmitStatusMetricsInterval <=0, metricsbp.SysStatsTickerInterval will be used as the fallback.
	//
	// Deprecated: Statsd metrics are deprecated.
	EmitStatusMetrics bool `yaml:"emitStatusMetrics"`
	// Deprecated: Statsd metrics are deprecated.
	EmitStatusMetricsInterval time.Duration `yaml:"emitStatusMetricsInterval"`

	// Logger is the logger to be called when the breaker changes states.
	//
	// Deprecated: We always log using slog with LogContext.
	Logger log.Wrapper `yaml:"logger"`

	// LogContext is the context to be used by slog to log when breaker changes
	// states.
	//
	// Optional, if not set, context.Background() will be used when logging.
	LogContext context.Context `yaml:"-"`

	// MaxRequestsHalfOpen represents he Maximum amount of requests that will be allowed through while the breaker
	// is in half-open state. If left unset (or set to 0), exactly 1 request will be allowed through while half-open.
	MaxRequestsHalfOpen uint32 `yaml:"maxRequestsHalfOpen"`

	// Interval represents the cyclical period of the 'Closed' state.
	// If 0, internal counts do not get reset while the breaker remains in the Closed state.
	Interval time.Duration `yaml:"interval"`

	// Timeout is the duration of the 'Open' state. After an 'Open' timeout duration has passed, the breaker enters 'half-open' state.
	Timeout time.Duration `yaml:"timeout"`

	// TimeoutJitterRatio is the jitter ratio to be applied to Timeout.
	//
	// Optional, default is 0.5, means the actual timeout used can be Timeout+-50%.
	TimeoutJitterRatio *float64 `yaml:"timeoutJitterRatio"`
}

// NewFailureRatioBreaker creates a new FailureRatioBreaker with the provided configuration.
func NewFailureRatioBreaker(config Config) FailureRatioBreaker {

	failureBreaker := FailureRatioBreaker{
		name:              config.Name,
		minRequestsToTrip: config.MinRequestsToTrip,
		failureThreshold:  config.FailureThreshold,
	}
	if config.LogContext != nil {
		failureBreaker.logContext = config.LogContext
	} else {
		failureBreaker.logContext = context.Background()
	}
	jitterRatio := 0.5
	if config.TimeoutJitterRatio != nil {
		jitterRatio = *config.TimeoutJitterRatio
		if jitterRatio <= 0 || jitterRatio > 1 {
			slog.WarnContext(
				failureBreaker.logContext,
				"Wrong breakerbp TimeoutJitterRatio config, will be normalized",
				"name", config.Name,
				"value", jitterRatio,
			)
		}
	}
	timeout := randbp.JitterDuration(config.Timeout, jitterRatio)
	breakerTimeout.With(prometheus.Labels{
		nameLabel: config.Name,
	}).Set(timeout.Seconds())
	slog.DebugContext(
		failureBreaker.logContext,
		"breakerbp jittered timeout",
		"name", config.Name,
		"timeout", timeout,
		"origin", config.Timeout,
		"jitterRatio", jitterRatio,
	)
	settings := gobreaker.Settings{
		Name:          config.Name,
		Interval:      config.Interval,
		Timeout:       timeout,
		MaxRequests:   config.MaxRequestsHalfOpen,
		ReadyToTrip:   failureBreaker.shouldTrip,
		OnStateChange: failureBreaker.stateChanged,
	}

	failureBreaker.goBreaker = gobreaker.NewCircuitBreaker(settings)

	breakerClosed.With(prometheus.Labels{
		nameLabel: config.Name,
	}).Set(1)

	return failureBreaker
}

// Execute wraps the given function call in circuit breaker logic and returns
// the result.
func (cb FailureRatioBreaker) Execute(fn func() (interface{}, error)) (interface{}, error) {
	return cb.goBreaker.Execute(fn)
}

// State returns the current state of the breaker.
func (cb FailureRatioBreaker) State() gobreaker.State {
	return cb.goBreaker.State()
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
			slog.WarnContext(
				cb.logContext,
				"tripping circuit breaker",
				"name", cb.name,
				"counts", counts,
			)
			return true
		}
	}
	return false
}

func (cb FailureRatioBreaker) stateChanged(name string, from gobreaker.State, to gobreaker.State) {
	var value float64
	if to != gobreaker.StateOpen {
		value = 1
	}
	breakerClosed.With(prometheus.Labels{
		nameLabel: cb.name,
	}).Set(value)

	slog.InfoContext(
		cb.logContext,
		"circuit breaker state changed",
		"name", name,
		"from", from,
		"to", to,
	)
}

var (
	_ CircuitBreaker = FailureRatioBreaker{}
	_ CircuitBreaker = (*gobreaker.CircuitBreaker)(nil)
)
