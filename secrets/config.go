package secrets

import (
	"context"
	"time"

	"github.com/reddit/baseplate.go/log"
)

// Config is the confuration struct for the secrets package.
//
// Can be deserialized from YAML.
type Config struct {
	// Path is the path to the secrets.json file file to load your service's
	// secrets from.
	Path string `yaml:"path"`
}

// InitFromConfig returns a new *secrets.Store using the given context and config.
func InitFromConfig(ctx context.Context, cfg Config) (*Store, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	store, err := NewStore(ctx, cfg.Path, log.ErrorWithSentryWrapper())
	if err != nil {
		return nil, err
	}
	return store, nil
}
