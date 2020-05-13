package redisbp

import (
	"fmt"
	"time"

	"github.com/go-redis/redis/v7"
)

// ClientConfig can be used to configure a redis-go "Client".  See the docs for
// redis.Options in redis-go for details on what each value means and what its
// defaults are:
// https://pkg.go.dev/github.com/go-redis/redis/v7?tab=doc#Options
//
// Can be deserialized from YAML.
type ClientConfig struct {
	// URL is passed to redis.ParseURL to initialize the client options.
	//
	// https://pkg.go.dev/github.com/go-redis/redis/v7?tab=doc#ParseURL
	URL string `yaml:"url"`

	Pool     PoolOptions    `yaml:"pool"`
	Retries  RetryOptions   `yaml:"retries"`
	Timeouts TimeoutOptions `yaml:"timeouts"`
}

// Options returns a redis.Options populated using the values from cfg.
func (cfg ClientConfig) Options() (*redis.Options, error) {
	options, err := redis.ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("redisbp: error parsing configured redis url. %w", err)
	}

	cfg.Pool.ApplyOptions(options)
	cfg.Retries.ApplyOptions(options)
	cfg.Timeouts.ApplyOptions(options)
	return options, nil
}

// ClusterConfig can be used to configure a redis-go "ClusterClient".  See the
// docs for redis.ClusterOptions in redis-go for details on what each value
// means and what its defaults are:
// https://pkg.go.dev/github.com/go-redis/redis/v7?tab=doc#ClusterOptions
//
// Can be deserialized from YAML.
type ClusterConfig struct {
	// Addrs is the seed list of cluster nodes in the format "host:port".
	//
	// Maps to Addrs on redis.ClusterClient.
	Addrs []string `yaml:"addrs"`

	Pool     PoolOptions    `yaml:"pool"`
	Retries  RetryOptions   `yaml:"retries"`
	Timeouts TimeoutOptions `yaml:"timeouts"`
}

// Options returns a redis.ClusterOptions populated using the values from cfg.
func (cfg ClusterConfig) Options() *redis.ClusterOptions {
	options := &redis.ClusterOptions{
		Addrs: cfg.Addrs,
	}

	cfg.Pool.ApplyClusterOptions(options)
	cfg.Retries.ApplyClusterOptions(options)
	cfg.Timeouts.ApplyClusterOptions(options)
	return options
}

// PoolOptions is used to configure the pool attributes of a redis-go Client or
// ClusterClient.  If any value is not set, it will use whatever default is
// defined by redis-go.
//
// See https://pkg.go.dev/github.com/go-redis/redis/v7?tab=doc#Options for details
// on the specific fields.
//
// Can be deserialized from YAML.
type PoolOptions struct {
	// Maps to PoolSize on the redis-go options.
	Size *int `yaml:"size"`

	// Maps to MinIdleConnections on the redis-go options.
	MinIdleConnections *int `yaml:"minIdleConnetions"`

	// Maps to MaxConnAge on the redis-go options.
	MaxConnectionAge *time.Duration `yaml:"maxConnectionAge"`

	// Maps to PoolTimeout on the redis-go options.
	Timeout *time.Duration `yaml:"timeout"`
}

// ApplyOptions applies the PoolOptions to the redis.Options.
func (opts PoolOptions) ApplyOptions(options *redis.Options) {
	if opts.MinIdleConnections != nil {
		options.MinIdleConns = *opts.MinIdleConnections
	}
	if opts.MaxConnectionAge != nil {
		options.MaxConnAge = *opts.MaxConnectionAge
	}
	if opts.Size != nil {
		options.PoolSize = *opts.Size
	}
	if opts.Timeout != nil {
		options.PoolTimeout = *opts.Timeout
	}
}

// ApplyClusterOptions applies the PoolOptions to the redis.ClusterOptions.
func (opts PoolOptions) ApplyClusterOptions(options *redis.ClusterOptions) {
	if opts.MinIdleConnections != nil {
		options.MinIdleConns = *opts.MinIdleConnections
	}
	if opts.MaxConnectionAge != nil {
		options.MaxConnAge = *opts.MaxConnectionAge
	}
	if opts.Size != nil {
		options.PoolSize = *opts.Size
	}
	if opts.Timeout != nil {
		options.PoolTimeout = *opts.Timeout
	}
}

// RetryOptions is used to configure the retry behavior of a redis-go Client or
// ClusterClient.
//
// See https://pkg.go.dev/github.com/go-redis/redis/v7?tab=doc#Options for details
// on the specific fields.
//
// Can be deserialized from YAML.
type RetryOptions struct {
	// Maps to MaxRetries on the redis-go options.
	Max *int `yaml:"max"`

	Backoff struct {
		// Maps to MinRetryBackoff on the redis-go options.
		Min *time.Duration `yaml:"min"`

		// Maps to MaxRetryBackoff on the redis-go options.
		Max *time.Duration `yaml:"max"`
	} `yaml:"backoff"`
}

// ApplyOptions applies the RetryOptions to the redis.Options.
func (opts RetryOptions) ApplyOptions(options *redis.Options) {
	if opts.Max != nil {
		options.MaxRetries = *opts.Max
	}
	if opts.Backoff.Min != nil {
		options.MinRetryBackoff = *opts.Backoff.Min
	}
	if opts.Backoff.Max != nil {
		options.MaxRetryBackoff = *opts.Backoff.Max
	}
}

// ApplyClusterOptions applies the RetryOptions to the redis.ClusterOptions.
func (opts RetryOptions) ApplyClusterOptions(options *redis.ClusterOptions) {
	if opts.Max != nil {
		options.MaxRetries = *opts.Max
	}
	if opts.Backoff.Min != nil {
		options.MinRetryBackoff = *opts.Backoff.Min
	}
	if opts.Backoff.Max != nil {
		options.MaxRetryBackoff = *opts.Backoff.Max
	}
}

// TimeoutOptions is used to configure the timeout behavior of a redis-go Client
// or ClusterClient.
//
// See https://pkg.go.dev/github.com/go-redis/redis/v7?tab=doc#Options for details
// on the specific fields.
//
// Can be deserialized from YAML.
type TimeoutOptions struct {
	Dial  *time.Duration `yaml:"dial"`
	Read  *time.Duration `yaml:"read"`
	Write *time.Duration `yaml:"write"`
}

// ApplyOptions applies the TimeoutOptions to the redis.Options.
func (opts TimeoutOptions) ApplyOptions(options *redis.Options) {
	if opts.Dial != nil {
		options.DialTimeout = *opts.Dial
	}
	if opts.Read != nil {
		options.ReadTimeout = *opts.Read
	}
	if opts.Write != nil {
		options.WriteTimeout = *opts.Write
	}
}

// ApplyClusterOptions applies the TimeoutOptions to the redis.ClusterOptions.
func (opts TimeoutOptions) ApplyClusterOptions(options *redis.ClusterOptions) {
	if opts.Dial != nil {
		options.DialTimeout = *opts.Dial
	}
	if opts.Read != nil {
		options.ReadTimeout = *opts.Read
	}
	if opts.Write != nil {
		options.WriteTimeout = *opts.Write
	}
}
