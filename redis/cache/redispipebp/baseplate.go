package redispipebp

import (
	"context"
	"errors"

	"github.com/avast/retry-go"
	"github.com/joomcode/redispipe/redis"
	"github.com/joomcode/redispipe/rediscluster"
	"github.com/joomcode/redispipe/redisconn"

	"github.com/reddit/baseplate.go/breakerbp"
	"github.com/reddit/baseplate.go/redis/cache/redisx"
)

// ErrBaseplateNameRequired is returned when do not set a Name in the argument struct to a new
// baseplate redis client.
var ErrBaseplateNameRequired = errors.New("redispipebp: Name is a required field in BaseplateRedisClientArgs")

// BaseplateRedisClientArgs is used to configure a baseplate redis client (cluster or single instance).
// The underlying client is wrapped in the following order, depending on the config:
//
//	- MonitoredSync
//	- If retrys are configured:
//		- RetrySync
//		- MonitoredSync for monitoring each request
//	- If a circuit breaker is configured:
//		- BreakerSync
//	- WrapErrorsSync
type BaseplateRedisClientArgs struct {
	// Name is the name of the redis client for use in client tracing.
	// This is a required field.
	Name string
	// Breaker is the circuit breaker config for the redis client.
	// This is optional, if it is nil then we will not wrap the client in a BreakerSync.
	Breaker *breakerbp.Config
	// Retry is the retry options for the redis client.
	// This is optional, if it is empty or nil, then we will not wrap the client in a RetrySync
	Retry []retry.Option
	// ReplicaPolicy is the config enumeration of policies of redis replica hosts usage.
	// This is optional, if nil, default to MasterOnly policy
	// This config only applies if NewBaseplateRedisCluster is used to create a redis client otherwise does nothing.
	// See: https://pkg.go.dev/github.com/joomcode/redispipe/rediscluster#ReplicaPolicyEnum
	ReplicaPolicy rediscluster.ReplicaPolicyEnum
}

func wrapSender(sender redis.Sender, args BaseplateRedisClientArgs) (s redisx.Sync, err error) {
	if args.Name == "" {
		err = ErrBaseplateNameRequired
		return
	}
	s = redisx.BaseSync{SyncCtx: redis.SyncCtx{S: sender}}
	s = WrapErrorsSync{s}
	if args.Breaker != nil {
		s = BreakerSync{
			Sync:    s,
			Breaker: breakerbp.NewFailureRatioBreaker(*args.Breaker),
		}
	}
	if len(args.Retry) != 0 {
		s = MonitoredSync{
			Sync: s,
			Name: args.Name + ".attempt",
		}
		s = RetrySync{
			Sync:        s,
			DefaultOpts: args.Retry,
		}
	}
	s = MonitoredSync{
		Sync: s,
		Name: args.Name,
	}
	return
}

// NewBaseplateRedisClient returns a new redisx.Sync that wraps a single instance redis client.
func NewBaseplateRedisClient(ctx context.Context, addr string, opts redisconn.Opts, args BaseplateRedisClientArgs) (redisx.Sync, error) {
	sender, err := redisconn.Connect(ctx, addr, opts)
	if err != nil {
		return nil, err
	}
	return wrapSender(sender, args)
}

// NewBaseplateRedisCluster returns a new redisx.Sync that wraps a single instance redis cluster client.
func NewBaseplateRedisCluster(ctx context.Context, addrs []string, opts rediscluster.Opts, args BaseplateRedisClientArgs) (redisx.Sync, error) {
	sender, err := rediscluster.NewCluster(ctx, addrs, opts)
	if err != nil {
		return nil, err
	}
	return wrapSender(sender.WithPolicy(args.ReplicaPolicy), args)
}
