package redisbp_test

import (
	"context"

	"github.com/reddit/baseplate.go"
	"github.com/reddit/baseplate.go/ecinterface"
	"github.com/reddit/baseplate.go/redis/db/redisbp"
)

type Config struct {
	baseplate.Config `yaml:",inline"`

	Redis redisbp.ClientConfig `yaml:"redis"`
}

// This example shows how you can embed a redis config in a struct and parse
// that with `baseplate.New`.
func ExampleClientConfig() {
	// In real code this MUST be replaced by the factory from the actual implementation.
	var ecFactory ecinterface.Factory

	var cfg Config
	if err := baseplate.ParseConfigYAML(&cfg); err != nil {
		panic(err)
	}

	ctx, bp, err := baseplate.New(context.Background(), baseplate.NewArgs{
		Config:             cfg,
		EdgeContextFactory: ecFactory,
	})
	if err != nil {
		panic(err)
	}
	defer bp.Close()

	client := redisbp.NewMonitoredClient("redis", redisbp.OptionsMust(cfg.Redis.Options()))
	client.Ping(ctx)
}
