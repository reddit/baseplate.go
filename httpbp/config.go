package httpbp

import retry "github.com/avast/retry-go"

// Config provides the configuration for a HTTP client including its
// middlewares.`
type Config struct {
	Slug           string               `yaml:"slug"`
	MaxConnections int                  `yaml:"maxConnections"`
	Retries        retry.Config         `yaml:"retries"`
	CircuitBreaker CircuitBreakerConfig `yaml:"circuitBreaker"`
	LoadBalancer   LoadBalancerConfig   `yaml:"loadBalancer"`
	MaxConcurrency int                  `yaml:"maxConcurrency"`
}

// LoadBalancerConfig provies the configuration for the load balancer client
// middleware implementing a round-robin load balancer.
type LoadBalancerConfig struct {
	Hosts []string `yaml:"hosts"`
}

// CircuitBreakerConfig provides the configuration for the circuit breaker
// client middleware.
type CircuitBreakerConfig struct {
	MinRequests     uint32  `yaml:"minRequests"`
	MinFailureRatio float64 `yaml:"minFailureRatio"`
}
