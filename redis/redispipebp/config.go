package redispipebp

import (
	"time"

	"github.com/joomcode/redispipe/rediscluster"
	"github.com/joomcode/redispipe/redisconn"
)

// ClientConfig can be used to configure a redispipe "Client".  See the docs for
// redisconn.Opts in redispipe for details on what each value means and what its
// defaults are:
// https://pkg.go.dev/github.com/joomcode/redispipe@v0.9.4/redisconn?tab=doc#Opts
//
// Can be deserialized from YAML.
//
// Examples:
//
// Minimal YAML:
//
//	redis:
//	 addr: localhost:6379
//
// Full YAML:
//
//	redis:
//	 addr: redis://localhost:6379
//	 opts:
//	  db: 1
//	  writePause: 150000ns # 150 microseconds
//	  reconnectPause: 2s
//	  tcpKeepAlive: 333ms
//	  asyncDial: false
//	  scriptMode: false
//	  timeouts:
//	   dial: 1s
//	   io: 500ms
type ClientConfig struct {
	// Addr is the address of your Redis server.
	//
	// This is the only required field.
	Addr string `yaml:"addr"`

	Options Opts `yaml:"options"`
}

// Opts returns a redisconn.Opts object with the configured options applied.
func (cfg ClientConfig) Opts() redisconn.Opts {
	opts := redisconn.Opts{}
	cfg.Options.ApplyOpts(&opts)
	return opts
}

// ClusterConfig can be used to configure a redispipe "Cluster".  See the docs for
// redisluster.Opts in redispipe.
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
//	 options:
//	  writePause: 150000ns # 150 microseconds
//	  reconnectPause: 2s
//	  tcpKeepAlive: 333ms
//	  asyncDial: false
//	  scriptMode: false
//	  timeouts:
//	   dial: 1s
//	   io: 500ms
type ClusterConfig struct {
	// Addrs is the list of seed addresses to connect to the Redis Cluster.
	Addrs []string `yaml:"addrs"`

	// Options maps to rediscluster.Opts.HostOpts
	Options Opts `yaml:"options"`

	// Cluster is the remaining, Redis Cluster specific options.
	Cluster ClusterOpts `yaml:"cluster"`
}

// Opts returns a rediscluster.Opts object with the configured options applied.
func (cfg ClusterConfig) Opts() rediscluster.Opts {
	opts := rediscluster.Opts{}
	cfg.Options.ApplyOpts(&opts.HostOpts)
	cfg.Cluster.ApplyOpts(&opts)
	return opts
}

// Opts is used to apply the configured options to a rediscluster.Opts.
//
// All fields in Opts are optional and redispipe provides default values.
//
// See the documentation for details on each field as well as the default values.
// https://pkg.go.dev/github.com/joomcode/redispipe@v0.9.4/redisconn?tab=doc#Opts
type Opts struct {
	// DB is the database number in Redis.
	//
	// Maps to redisconn.Opts.DB
	DB *int `yaml:"db"`

	// WritePause is the duration of time the write loop should wait to collect
	// commands to flush.
	//
	// Maps to redisconn.Opts.WritePause
	WritePause time.Duration `yaml:"writePause"`

	// ReconnectPause is how long the client will wait to attempt to make a
	// connection the previous attempt fails.
	//
	// Maps to redisconn.Opts.ReconnectPause
	ReconnectPause time.Duration `yaml:"reconnectPause"`

	// TCPKeepAlive is the KeepAlive parameter for the net.Dial used to make a new
	// connection.
	//
	// Maps to redisconn.Opts.TCPKeepAlive
	TCPKeepAlive time.Duration `yaml:"tcpKeepAlive"`

	// Timeouts is used to configure the different connection timeouts.
	Timeouts Timeouts `yaml:"timeouts"`
}

// ApplyOpts applies any values set in o to opts, if a value is not set on o, then
// we don't update opts and allow redispipe to use its defaults.
func (o Opts) ApplyOpts(opts *redisconn.Opts) {

	if o.DB != nil {
		opts.DB = *o.DB
	}
	if o.WritePause > 0 {
		opts.WritePause = o.WritePause
	}
	if o.ReconnectPause > 0 {
		opts.ReconnectPause = o.ReconnectPause
	}
	if o.TCPKeepAlive > 0 {
		opts.TCPKeepAlive = o.TCPKeepAlive
	}
	o.Timeouts.ApplyOpts(opts)
}

// Timeouts applies the timeout options to redisconn.Opts.
//
// All fields in Timeouts are optional and redispipe provides default values.
//
// Can be deserialized from YAML.
type Timeouts struct {
	// Dial is the timeout for the net.Dialer used to create a connection.
	//
	// Maps to redisconn.Opts.DialTimeout
	Dial time.Duration `yaml:"dial"`

	// IO is the timeout for read/write operations on the socket.
	//
	// Maps to redisconn.Opts.IOTimeout
	IO time.Duration `yaml:"io"`
}

// ApplyOpts applies any values set in t to opts, if a value is not set on t, then
// we don't update opts and allow redispipe to use its defaults.
func (t Timeouts) ApplyOpts(opts *redisconn.Opts) {
	if t.Dial > 0 {
		opts.DialTimeout = t.Dial
	}
	if t.IO > 0 {
		opts.IOTimeout = t.IO
	}
}

// ClusterOpts are Redis Cluster specific options.
//
// All fields in Opts are optional and redispipe provides default values.
//
// See the documentation for details on each field as well as the default values.
// https://pkg.go.dev/github.com/joomcode/redispipe@v0.9.4/rediscluster?tab=doc#Opts
type ClusterOpts struct {
	// Name is the name of the cluster.
	//
	// Maps to rediscluster.Opts.Name
	Name string `yaml:"name"`
}

// ApplyOpts applies any values set in o to opts, if a value is not set on o, then
// we don't update opts and allow redispipe to use its defaults.
func (o ClusterOpts) ApplyOpts(opts *rediscluster.Opts) {
	if o.Name != "" {
		opts.Name = o.Name
	}
}
