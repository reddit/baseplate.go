package redisbp

import (
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// ClientConfig can be used to configure a redis-go "Client".  See the docs for
// redis.Options in redis-go for details on what each value means and what its
// defaults are:
// https://pkg.go.dev/github.com/go-redis/redis/v8?tab=doc#Options
//
// Can be deserialized from YAML.
//
// Examples:
//
// Minimal YAML:
//
//	redis:
//	 url: redis://localhost:6379
//
// Full YAML:
//
//	redis:
//	 url: redis://localhost:6379
//	 pool:
//	  size: 10
//	  minIdleConnections: 5
//	  maxConnectionAge: 1m
//	  timeout: 10s
//	 retries:
//	  max: 2
//	  minBackoff: 1ms
//	  maxBackoff: 10ms
//	 timeouts:
//	  dial: 1s
//	  read: 100ms
//	  write: 200ms
type ClientConfig struct {
	// URL is passed to redis.ParseURL to initialize the client options.  This is
	// a required field.
	//
	// https://pkg.go.dev/github.com/go-redis/redis/v8?tab=doc#ParseURL
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
// https://pkg.go.dev/github.com/go-redis/redis/v8?tab=doc#ClusterOptions
//
// Can be deserialized from YAML.
//
// Examples:
//
// Minimal YAML:
//
//	redis:
//	 addrs:
//	  - localhost:6379
//	  - localhost:6380
//
// Full YAML:
//
//	redis:
//	 addrs:
//	  - localhost:6379
//	  - localhost:6380
//	 pool:
//	  size: 10
//	  minIdleConnections: 5
//	  maxConnectionAge: 1m
//	  timeout: 10s
//	 retries:
//	  max: 2
//	  minBackoff: 1ms
//	  maxBackoff: 10ms
//	 timeouts:
//	  dial: 1s
//	  read: 100ms
//	  write: 200ms
type ClusterConfig struct {
	// Addrs is the seed list of cluster nodes in the format "host:port". This is
	// a required field.
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
// See https://pkg.go.dev/github.com/go-redis/redis/v8?tab=doc#Options for details
// on the specific fields.
//
// Can be deserialized from YAML.
type PoolOptions struct {
	// Maps to PoolSize on the redis-go options.
	Size int `yaml:"size"`

	// Maps to MinIdleConnections on the redis-go options.
	MinIdleConnections int `yaml:"minIdleConnections"`

	// Maps to MaxConnAge on the redis-go options.
	MaxConnectionAge time.Duration `yaml:"maxConnectionAge"`

	// Maps to PoolTimeout on the redis-go options.
	Timeout time.Duration `yaml:"timeout"`
}

// ApplyOptions applies the PoolOptions to the redis.Options.
func (opts PoolOptions) ApplyOptions(options *redis.Options) {
	if opts.MinIdleConnections != 0 {
		options.MinIdleConns = opts.MinIdleConnections
	}
	if opts.MaxConnectionAge != 0 {
		options.MaxConnAge = opts.MaxConnectionAge
	}
	if opts.Size != 0 {
		options.PoolSize = opts.Size
	}
	if opts.Timeout != 0 {
		options.PoolTimeout = opts.Timeout
	}
}

// ApplyClusterOptions applies the PoolOptions to the redis.ClusterOptions.
func (opts PoolOptions) ApplyClusterOptions(options *redis.ClusterOptions) {
	if opts.MinIdleConnections != 0 {
		options.MinIdleConns = opts.MinIdleConnections
	}
	if opts.MaxConnectionAge != 0 {
		options.MaxConnAge = opts.MaxConnectionAge
	}
	if opts.Size != 0 {
		options.PoolSize = opts.Size
	}
	if opts.Timeout != 0 {
		options.PoolTimeout = opts.Timeout
	}
}

// RetryOptions is used to configure the retry behavior of a redis-go Client or
// ClusterClient.
//
// See https://pkg.go.dev/github.com/go-redis/redis/v8?tab=doc#Options for details
// on the specific fields.
//
// Can be deserialized from YAML.
type RetryOptions struct {
	// Maps to MaxRetries on the redis-go options.
	Max int `yaml:"max"`

	// Maps to MinRetryBackoff on the redis-go options.
	MinBackoff time.Duration `yaml:"minBackoff"`

	// Maps to MaxRetryBackoff on the redis-go options.
	MaxBackoff time.Duration `yaml:"maxBackoff"`
}

// ApplyOptions applies the RetryOptions to the redis.Options.
func (opts RetryOptions) ApplyOptions(options *redis.Options) {
	if opts.Max != 0 {
		options.MaxRetries = opts.Max
	}
	if opts.MinBackoff != 0 {
		options.MinRetryBackoff = opts.MinBackoff
	}
	if opts.MaxBackoff != 0 {
		options.MaxRetryBackoff = opts.MaxBackoff
	}
}

// ApplyClusterOptions applies the RetryOptions to the redis.ClusterOptions.
func (opts RetryOptions) ApplyClusterOptions(options *redis.ClusterOptions) {
	if opts.Max != 0 {
		options.MaxRetries = opts.Max
	}
	if opts.MinBackoff != 0 {
		options.MinRetryBackoff = opts.MinBackoff
	}
	if opts.MaxBackoff != 0 {
		options.MaxRetryBackoff = opts.MaxBackoff
	}
}

// TimeoutOptions is used to configure the timeout behavior of a redis-go Client
// or ClusterClient.
//
// See https://pkg.go.dev/github.com/go-redis/redis/v8?tab=doc#Options for details
// on the specific fields.
//
// Can be deserialized from YAML.
type TimeoutOptions struct {
	Dial  time.Duration `yaml:"dial"`
	Read  time.Duration `yaml:"read"`
	Write time.Duration `yaml:"write"`
}

// ApplyOptions applies the TimeoutOptions to the redis.Options.
func (opts TimeoutOptions) ApplyOptions(options *redis.Options) {
	if opts.Dial != 0 {
		options.DialTimeout = opts.Dial
	}
	if opts.Read != 0 {
		options.ReadTimeout = opts.Read
	}
	if opts.Write != 0 {
		options.WriteTimeout = opts.Write
	}
}

// ApplyClusterOptions applies the TimeoutOptions to the redis.ClusterOptions.
func (opts TimeoutOptions) ApplyClusterOptions(options *redis.ClusterOptions) {
	if opts.Dial != 0 {
		options.DialTimeout = opts.Dial
	}
	if opts.Read != 0 {
		options.ReadTimeout = opts.Read
	}
	if opts.Write != 0 {
		options.WriteTimeout = opts.Write
	}
}

// OptionsMust can be combine with ClientOptions.Options() to either return
// the *redis.Options object or panic if an error was returned.  This allows
// you to just pass this into redis.NewClient.
//
// Ex:
//
//	var opts redisbp.ClientOptions
//	client := redis.NewClient(redisbp.OptionsMust(opts.Options()))
func OptionsMust(options *redis.Options, err error) *redis.Options {
	if err != nil {
		panic(err)
	}
	return options
}
