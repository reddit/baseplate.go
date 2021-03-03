package redispipebp_test

import (
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/joomcode/redispipe/rediscluster"
	"github.com/joomcode/redispipe/redisconn"
	"gopkg.in/yaml.v2"

	"github.com/reddit/baseplate.go/redis/redispipebp"
)

func TestClientConfig(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		raw  string
		addr string
		opts redisconn.Opts
	}{
		{
			name: "addr-only",
			raw: `
addr: localhost:6379
`,
			addr: "localhost:6379",
			opts: redisconn.Opts{},
		},
		{
			name: "all",
			raw: `
addr: localhost:6379
options:
 db: 1
 writePause: 100µs
 reconnectPause: 1s
 tcpKeepAlive: 5m
 timeouts:
  dial: 1s
  io: 100ms
`,
			addr: "localhost:6379",
			opts: redisconn.Opts{
				DB:             1,
				WritePause:     100 * time.Microsecond,
				TCPKeepAlive:   5 * time.Minute,
				ReconnectPause: time.Second,
				DialTimeout:    time.Second,
				IOTimeout:      100 * time.Millisecond,
			},
		},
	}
	for _, _c := range cases {
		c := _c
		t.Run(c.name, func(t *testing.T) {
			var cfg redispipebp.ClientConfig
			if err := yaml.NewDecoder(strings.NewReader(c.raw)).Decode(&cfg); err != nil {
				t.Fatal(err)
			}
			if cfg.Addr != c.addr {
				t.Errorf("address mismatch, expected %q, got %q", c.addr, cfg.Addr)
			}

			opts := cfg.Opts()

			if !reflect.DeepEqual(c.opts, opts) {
				t.Errorf("redis.Opts mismatch\n\nexpected %#v\n\ngot %#v\n\n", c.opts, opts)
			}
		})
	}
}

func TestClusterConfig(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		raw   string
		addrs []string
		opts  rediscluster.Opts
	}{
		{
			name: "addrs-only",
			raw: `
addrs:
 - localhost:6379
 - localhost:6380
`,
			addrs: []string{"localhost:6379", "localhost:6380"},
			opts:  rediscluster.Opts{},
		},
		{
			name: "all",
			raw: `
addrs:
 - localhost:6379
 - localhost:6380
cluster:
 name: foo
options:
 db: 1
 writePause: 100µs
 reconnectPause: 1s
 tcpKeepAlive: 5m
 timeouts:
  dial: 1s
  io: 100ms
`,
			addrs: []string{"localhost:6379", "localhost:6380"},
			opts: rediscluster.Opts{
				Name: "foo",
				HostOpts: redisconn.Opts{
					DB:             1,
					WritePause:     100 * time.Microsecond,
					TCPKeepAlive:   5 * time.Minute,
					ReconnectPause: time.Second,
					DialTimeout:    time.Second,
					IOTimeout:      100 * time.Millisecond,
				},
			},
		},
	}
	for _, _c := range cases {
		c := _c
		t.Run(c.name, func(t *testing.T) {
			var cfg redispipebp.ClusterConfig
			if err := yaml.NewDecoder(strings.NewReader(c.raw)).Decode(&cfg); err != nil {
				t.Fatal(err)
			}
			sort.Strings(cfg.Addrs)
			sort.Strings(c.addrs)
			if !reflect.DeepEqual(cfg.Addrs, c.addrs) {
				t.Errorf("address mismatch, expected %+v, got %+v", c.addrs, cfg.Addrs)
			}

			opts := cfg.Opts()

			if !reflect.DeepEqual(c.opts, opts) {
				t.Errorf("redis.Opts mismatch\n\nexpected %#v\n\ngot %#v\n\n", c.opts, opts)
			}
		})
	}
}
