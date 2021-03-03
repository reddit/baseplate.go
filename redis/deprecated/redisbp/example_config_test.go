package redisbp_test

import (
	"context"

	"github.com/go-redis/redis/v7"
	"github.com/reddit/baseplate.go"
	"github.com/reddit/baseplate.go/ecinterface"
	"github.com/reddit/baseplate.go/redis/deprecated/redisbp"
)

type Config struct {
	Redis redisbp.ClientConfig `yaml:"redis"`
}

// This example shows how you can embed a redis config in a struct and parse
// that with `baseplate.New`.
func ExampleClientConfig() {
	var cfg Config
	// In real code this MUST be replaced by the factory from the actual implementation.
	var ecFactory ecinterface.Factory
	ctx, bp, err := baseplate.New(context.Background(), baseplate.NewArgs{
		ConfigPath:         "example.yaml",
		EdgeContextFactory: ecFactory,
		ServiceCfg:         &cfg,
	})
	if err != nil {
		panic(err)
	}
	defer bp.Close()

	factory := redisbp.NewMonitoredClientFactory(
		"redis",
		redis.NewClient(redisbp.OptionsMust(cfg.Redis.Options())),
	)
	client := factory.BuildClient(ctx)
	client.Ping()
}
