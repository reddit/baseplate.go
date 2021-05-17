package httpbp

import "github.com/reddit/baseplate.go/breakerbp"

// ClientConfig provides the configuration for a HTTP client including its
// middlewares.
type ClientConfig struct {
	Slug              string            `yaml:"slug"`
	MaxErrorReadAhead int               `yaml:"limitErrorReading"`
	MaxConcurrency    int               `yaml:"maxConcurrency"`
	MaxConnections    int               `yaml:"maxConnections"`
	CircuitBreaker    *breakerbp.Config `yaml:"circuitBreaker"`
}
