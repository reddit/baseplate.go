package baseplate

import (
	"errors"
	"io"
	"os"
	"time"

	yaml "gopkg.in/yaml.v2"

	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/secrets"
)

// ServerConfig is a general purpose config for assembling a BaseplateServer
type ServerConfig struct {
	Addr string

	Timeout time.Duration

	Log struct {
		Level log.Level
	}

	Metrics struct {
		Namespace           string
		Endpoint            string
		CounterSampleRate   float64
		HistogramSampleRate float64
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
		RecordTimeout time.Duration
		SampleRate    float64
	}
}

// Server is the primary interface for baseplate servers.
type Server interface {
	io.Closer

	Config() ServerConfig
	Secrets() *secrets.Store
	Serve() error
}

// ParseServerConfig will populate a ServerConfig from a YAML file.
func ParseServerConfig(path string) (*ServerConfig, error) {
	cfg := &ServerConfig{}
	if path == "" {
		return nil, errors.New("no config path given")
	}

	reader, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	decoder := yaml.NewDecoder(reader)
	if err = decoder.Decode(cfg); err != nil {
		return nil, err
	}

	log.Debugf("%#v", cfg)
	return cfg, nil
}
