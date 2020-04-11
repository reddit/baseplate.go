package baseplate

import (
	"errors"
	"io/ioutil"
	"time"

	"github.com/reddit/baseplate.go/log"
	"gopkg.in/yaml.v2"
)

// ServerConfig is a general purpose config for assembling a BaseplateServer
type ServerConfig struct {
	Addr string

	Timeout time.Duration

	Log struct {
		Level log.Level
	}

	Metrics struct {
		Namespace string
		Endpoint  string
	}

	Secrets struct {
		Path string
	}

	Sentry struct {
		DSN         string
		Environment string
		SampleRate  float64
	}

	Tracing struct {
		Namespace     string
		Endpoint      string
		RecordTimeout time.Duration `yaml:"recordTimeout"`
		SampleRate    float64
	}
}

// Server is the primary interface for baseplate servers.
type Server interface {
	Config() ServerConfig
	Impl() interface{}
	Serve() error
	Stop() error
}

// ParseServerConfig will populate a ServerConfig from a YAML file.
func ParseServerConfig(path string, cfg interface{}) error {
	if path == "" {
		return errors.New("no config path given")
	}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	if err = yaml.Unmarshal(data, &cfg); err != nil {
		return err
	}

	log.Debugf("%#v", cfg)
	return nil
}
