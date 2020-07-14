package redisbp_test

import (
	"context"

	"github.com/reddit/baseplate.go"
	"github.com/reddit/baseplate.go/redisbp"
)

type config struct {
	Redis redisbp.ClientConfig `yaml:"redis"`
}

// This example shows how you can embed a redis config in a struct and parse
// that with `baseplate.New`.
func ExampleClientConfig() {
	var cfg config
	ctx, bp, err := baseplate.New(context.Background(), "example.yaml", &cfg)
	if err != nil {
		panic(err)
	}
	defer bp.Close()

	client := redisbp.NewMonitoredClient("redis", redisbp.OptionsMust(cfg.Redis.Options()))
	client.Ping(ctx)
}
