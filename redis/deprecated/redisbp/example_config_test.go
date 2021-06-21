package redisbp_test

import (
	"context"

	"github.com/reddit/baseplate.go"
	"github.com/reddit/baseplate.go/redis/deprecated/redisbp"
)

type Config struct {
	baseplate.Config `yaml:",inline"`

	Redis redisbp.ClientConfig `yaml:"redis"`
}

// This example shows how you can embed a redis config in a struct and parse
// that with `baseplate.New`.
func ExampleClientConfig() {
	var cfg Config
	ctx, bp, err := baseplate.New(context.Background(), baseplate.NewArgs{
		ConfigPath: "example.yaml",
		ServiceCfg: &cfg,
	})
	if err != nil {
		panic(err)
	}
	defer bp.Close()

	client := redisbp.NewMonitoredClient("redis", redisbp.OptionsMust(cfg.Redis.Options()))
	client.Ping(ctx)
}
