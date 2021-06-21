package redisbp_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"gopkg.in/yaml.v2"

	"github.com/reddit/baseplate.go/redis/deprecated/redisbp"
)

func TestClientConfig(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		raw      string
		expected redisbp.ClientConfig
		options  *redis.Options
	}{
		{
			name: "url-only",
			raw: `
url: redis://localhost:6379
`,
			expected: redisbp.ClientConfig{
				URL: "redis://localhost:6379",
			},
			options: &redis.Options{
				Network: "tcp",
				Addr:    "localhost:6379",
			},
		},
		{
			name: "all",
			raw: `
url: redis://localhost:6379
pool:
 size: 10
 minIdleConnections: 5
 maxConnectionAge: 1m
 timeout: 10s
retries:
 max: 2
 minBackoff: 1ms
 maxBackoff: 10ms
timeouts:
 dial: 1s
 read: 100ms
 write: 200ms
`,
			expected: redisbp.ClientConfig{
				URL: "redis://localhost:6379",

				Pool: redisbp.PoolOptions{
					Size:               10,
					MinIdleConnections: 5,
					MaxConnectionAge:   time.Minute,
					Timeout:            time.Second * 10,
				},

				Retries: redisbp.RetryOptions{
					Max:        2,
					MinBackoff: time.Millisecond,
					MaxBackoff: time.Millisecond * 10,
				},

				Timeouts: redisbp.TimeoutOptions{
					Dial:  time.Second,
					Read:  time.Millisecond * 100,
					Write: time.Millisecond * 200,
				},
			},
			options: &redis.Options{
				Network: "tcp",
				Addr:    "localhost:6379",

				MinIdleConns: 5,
				MaxConnAge:   time.Minute,
				PoolSize:     10,
				PoolTimeout:  time.Second * 10,

				MaxRetries:      2,
				MinRetryBackoff: time.Millisecond,
				MaxRetryBackoff: time.Millisecond * 10,

				DialTimeout:  time.Second,
				ReadTimeout:  time.Millisecond * 100,
				WriteTimeout: time.Millisecond * 200,
			},
		},
	}

	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				var cfg redisbp.ClientConfig
				if err := yaml.NewDecoder(strings.NewReader(c.raw)).Decode(&cfg); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(c.expected, cfg) {
					t.Errorf("client config mismatch:\n\nexpected %#v\n\ngot %#v\n\n", c.expected, cfg)
				}

				options, err := cfg.Options()
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(c.options, options) {
					t.Errorf("redis.Options mismatch\n\nexpected %#v\n\ngot %#v\n\n", c.options, options)
				}
			},
		)
	}
}

func TestClusterClientConfig(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		raw      string
		expected redisbp.ClusterConfig
		options  *redis.ClusterOptions
	}{
		{
			name: "url-only",
			raw: `
addrs:
 - localhost:6379
 - localhost:6380
`,
			expected: redisbp.ClusterConfig{
				Addrs: []string{"localhost:6379", "localhost:6380"},
			},
			options: &redis.ClusterOptions{
				Addrs: []string{"localhost:6379", "localhost:6380"},
			},
		},
		{
			name: "all",
			raw: `
addrs:
 - localhost:6379
 - localhost:6380
pool:
 size: 10
 minIdleConnections: 5
 maxConnectionAge: 1m
 timeout: 10s
retries:
 max: 2
 minBackoff: 1ms
 maxBackoff: 10ms
timeouts:
 dial: 1s
 read: 100ms
 write: 200ms
`,
			expected: redisbp.ClusterConfig{
				Addrs: []string{"localhost:6379", "localhost:6380"},

				Pool: redisbp.PoolOptions{
					Size:               10,
					MinIdleConnections: 5,
					MaxConnectionAge:   time.Minute,
					Timeout:            time.Second * 10,
				},

				Retries: redisbp.RetryOptions{
					Max:        2,
					MinBackoff: time.Millisecond,
					MaxBackoff: time.Millisecond * 10,
				},

				Timeouts: redisbp.TimeoutOptions{
					Dial:  time.Second,
					Read:  time.Millisecond * 100,
					Write: time.Millisecond * 200,
				},
			},
			options: &redis.ClusterOptions{
				Addrs: []string{"localhost:6379", "localhost:6380"},

				MinIdleConns: 5,
				MaxConnAge:   time.Minute,
				PoolSize:     10,
				PoolTimeout:  time.Second * 10,

				MaxRetries:      2,
				MinRetryBackoff: time.Millisecond,
				MaxRetryBackoff: time.Millisecond * 10,

				DialTimeout:  time.Second,
				ReadTimeout:  time.Millisecond * 100,
				WriteTimeout: time.Millisecond * 200,
			},
		},
	}

	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				var cfg redisbp.ClusterConfig
				if err := yaml.NewDecoder(strings.NewReader(c.raw)).Decode(&cfg); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(c.expected, cfg) {
					t.Errorf("client config mismatch:\n\nexpected %#v\n\ngot %#v\n\n", c.expected, cfg)
				}

				options := cfg.Options()
				if !reflect.DeepEqual(c.options, options) {
					t.Errorf("redis.Options mismatch\n\nexpected %#v\n\ngot %#v\n\n", c.options, options)
				}
			},
		)
	}
}
