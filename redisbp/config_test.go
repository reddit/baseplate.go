package redisbp_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/go-redis/redis/v7"
	"github.com/reddit/baseplate.go/redisbp"
	"gopkg.in/yaml.v2"
)

type clientConfig struct {
	Redis redisbp.ClientConfig
}

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
redis:
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
	}

	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				var cfg clientConfig
				if err := yaml.NewDecoder(strings.NewReader(c.raw)).Decode(&cfg); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(c.expected, cfg.Redis) {
					t.Errorf("client config mismatch, expected %#v, got %#v", c.expected, cfg.Redis)
				}

				options, err := cfg.Redis.Options()
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(c.options, options) {
					t.Errorf("client config mismatch, expected %#v, got %#v", c.options, options)
				}
			},
		)
	}
}
