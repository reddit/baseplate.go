package httpbp

// ClientConfig provides the configuration for a HTTP client including its
// middlewares.
type ClientConfig struct {
	Slug           string               `yaml:"slug"`
	MaxConnections int                  `yaml:"maxConnections"`
	CircuitBreaker CircuitBreakerConfig `yaml:"circuitBreaker"`
	MaxConcurrency int                  `yaml:"maxConcurrency"`
}

// CircuitBreakerConfig provides the configuration for the circuit breaker
// client middleware.
type CircuitBreakerConfig struct {
	MinRequests     uint32  `yaml:"minRequests"`
	MinFailureRatio float64 `yaml:"minFailureRatio"`
}
