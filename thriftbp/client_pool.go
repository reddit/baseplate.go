package thriftbp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	retry "github.com/avast/retry-go"
	"github.com/go-kit/kit/metrics"

	"github.com/reddit/baseplate.go/breakerbp"
	"github.com/reddit/baseplate.go/clientpool"
	"github.com/reddit/baseplate.go/errorsbp"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/metricsbp"
)

// DefaultPoolGaugeInterval is the fallback value to be used when
// ClientPoolConfig.PoolGaugeInterval <= 0.
const DefaultPoolGaugeInterval = time.Second * 10

// PoolError is returned by ClientPool.Call when it fails to get a client from
// its pool.
type PoolError struct {
	// Cause is the inner error wrapped by PoolError.
	Cause error
}

func (err PoolError) Error() string {
	return "thriftpb: error getting a client from the pool: " + err.Cause.Error()
}

func (err PoolError) Unwrap() error {
	return err.Cause
}

var (
	_ error = PoolError{}
	_ error = (*PoolError)(nil)
)

// ClientPoolConfig is the configuration struct for creating a new ClientPool.
type ClientPoolConfig struct {
	// ServiceSlug is a short identifier for the thrift service you are creating
	// clients for.  The preferred convention is to take the service's name,
	// remove the 'Service' prefix, if present, and convert from camel case to
	// all lower case, hyphen separated.
	//
	// Examples:
	//
	//     AuthenticationService -> authentication
	//     ImageUploadService -> image-upload
	ServiceSlug string

	// Addr is the address of a thrift service.  Addr must be in the format
	// "${host}:${port}"
	Addr string

	// InitialConnections is the inital number of thrift connections created by
	// the client pool.
	InitialConnections int

	// MaxConnections is the maximum number of thrift connections the client
	// pool can maintain.
	MaxConnections int

	// MaxConnectionAge is the maximum duration that a pooled connection will be
	// kept before closing in favor of a new one.
	//
	// If this is not set, the default duration is 5 minutes.
	//
	// To disable this and keep connections in the pool indefinetly, set this to
	// a negative value.
	MaxConnectionAge time.Duration

	// ConnectTimeout and SocketTimeout are timeouts used by the underlying
	// thrift.TSocket.
	//
	// In most cases, you would want ConnectTimeout to be short, because if you
	// have problem connecting to the upstream you want to fail fast.
	//
	// For SocketTimeout, the value you should set depends on whether you set a
	// deadline to the context object to the client call functions or not.
	// If ALL your client calls will have a context object with a deadline
	// attached, then it's recommended to set SocketTimeout to a short value,
	// as this is the max overhead the client call will take over the set
	// deadline, in case the server is not-responding.
	// But if you don't always have a deadline attached to your client calls,
	// then SocketTimeout needs to be at least your upstream service's p99 latency
	// SLA. If it's shorter than that you are gonna close connections and fail
	// requests prematurely.
	//
	// It's recommended to make sure all your client call context objects have a
	// deadline set, and set SocketTimeout to a short value. For example:
	//
	//     clientCtx, cancel := context.WithTimeout(ctx, myCallTimeout)
	//     defer cancel()
	//     resp, err := client.MyCall(clientCtx, args)
	//
	// For both values, <=0 would mean no timeout.
	// In most cases you would want to set timeouts for both.
	ConnectTimeout time.Duration
	SocketTimeout  time.Duration

	// Any tags that should be applied to metrics logged by the ClientPool.
	// This includes the optional pool stats.
	MetricsTags metricsbp.Tags

	// DefaultRetryOptions is the list of retry.Options to apply as the defaults
	// for the Retry middleware.
	//
	// This is optional, if it is not set, we will use a single option,
	// retry.Attempts(1).  This sets up the retry middleware but does not
	// automatically retry any requests.  You can set retry behavior per-call by
	// using retrybp.WithOptions.
	DefaultRetryOptions []retry.Option

	// ReportPoolStats signals to the ClientPool that it should report
	// statistics on the underlying clientpool.Pool in a background
	// goroutine.  If this is set to false, the reporting goroutine will
	// not be started and it will not report pool stats.
	//
	// It reports:
	// - the number of active clients to a gauge named
	//   "${ServiceSlug}.pool-active-connections".
	// - the number of allocated clients to a gauge named
	//   "${ServiceSlug}.pool-allocated-clients".
	//
	// The reporting goroutine is cancelled when the global metrics client
	// context is Done.
	ReportPoolStats bool

	// PoolGaugeInterval indicates how often we should update the active
	// connections gauge when collecting pool stats.
	//
	// When PoolGaugeInterval <= 0 and ReportPoolStats is true,
	// DefaultPoolGaugeInterval will be used instead.
	PoolGaugeInterval time.Duration

	// Suppress some of the errors returned by the server before sending them to
	// the client span.
	//
	// See MonitorClientArgs.ErrorSpanSuppressor for more details.
	//
	// This is optional. If it's not set none of the errors will be suppressed.
	ErrorSpanSuppressor errorsbp.Suppressor

	// When InitialConnectionsFallback is set to true and an error occurred when
	// we try to initialize the client pool, instead of returning that error,
	// we try again with InitialConnections falls back to 0.
	//
	// If the fallback attempt succeeded, we log the initial error with
	// InitialConnectionsFallbackLogger, and returns nil error.
	// If the fallback attempt also failed, we return both errors.
	//
	// This is useful when the server is unstable that some connection errors are
	// expected, so that we still try to create InitialConnections when possible,
	// but returns an usable client pool with 0 initial connections as fallback.
	InitialConnectionsFallback       bool
	InitialConnectionsFallbackLogger log.Wrapper

	// When BreakerConfig is non-nil,
	// a breakerbp.FailureRatioBreaker will be created for the pool,
	// and its middleware will be set for the pool.
	BreakerConfig *breakerbp.Config
}

// Client is a client object that implements both the clientpool.Client and
// thrift.TCLient interfaces.
//
// This allows it to be managed by a clientpool.Pool and be passed to a thrift
// client as the base thrift.TClient.
type Client interface {
	clientpool.Client
	thrift.TClient
}

// ClientPool defines an object that implements thrift.TClient using a pool of
// Client objects.
type ClientPool interface {
	// ClientPool implements TClient by grabbing a Client from it's pool and
	// releasing that Client after it's Call method completes.
	//
	// If Call fails to get a client from the pool, it will return PoolError.
	// You can check the error returned by Call using:
	//
	//     var poolErr thriftbp.PoolError
	//     if errors.As(err, &poolErr) {
	//       // It's unable to get a client from the pool
	//     } else {
	//       // It's error from the actual thrift call
	//     }
	//
	// If the error is not of type PoolError that means it's returned by the
	// Call from the actual client.
	//
	// If Call fails to release the client back to the pool,
	// it will log the error on error level but not return it to the caller.
	// It also increase ServiceSlug+".pool-release-error" counter.
	thrift.TClient

	// Passthrough APIs from clientpool.Pool:
	io.Closer
	IsExhausted() bool
}

// AddressGenerator defines a function that returns the address of a thrift
// service.
//
// Services should generally not have to use AddressGenerators directly,
// instead you should use NewBaseplateClientPool which uses the default
// AddressGenerator for a typical Baseplate Thrift Client.
type AddressGenerator func() (string, error)

// SingleAddressGenerator returns an AddressGenerator that always returns addr.
//
// Services should generally not have to use SingleAddressGenerator
// directly, instead you should use NewBaseplateClientPool which uses the
// default AddressGenerator for a typical Baseplate Thrift Client.
func SingleAddressGenerator(addr string) AddressGenerator {
	return func() (string, error) {
		return addr, nil
	}
}

// NewBaseplateClientPool returns a standard ClientPool wrapped with the
// BaseplateDefaultClientMiddlewares plus any additional client middlewares
// passed into this function.
func NewBaseplateClientPool(cfg ClientPoolConfig, middlewares ...thrift.ClientMiddleware) (ClientPool, error) {
	defaults := BaseplateDefaultClientMiddlewares(
		DefaultClientMiddlewareArgs{
			ServiceSlug:         cfg.ServiceSlug,
			RetryOptions:        cfg.DefaultRetryOptions,
			ErrorSpanSuppressor: cfg.ErrorSpanSuppressor,
			BreakerConfig:       cfg.BreakerConfig,
		},
	)
	middlewares = append(middlewares, defaults...)
	return NewCustomClientPool(
		cfg,
		SingleAddressGenerator(cfg.Addr),
		thrift.NewTHeaderProtocolFactory(),
		middlewares...,
	)
}

// NewCustomClientPool creates a ClientPool that uses a custom AddressGenerator
// and TProtocolFactory wrapped with the given middleware.
//
// Most services will want to just use NewBaseplateClientPool, this has been
// provided to support services that have non-standard and/or legacy needs.
func NewCustomClientPool(
	cfg ClientPoolConfig,
	genAddr AddressGenerator,
	protoFactory thrift.TProtocolFactory,
	middlewares ...thrift.ClientMiddleware,
) (ClientPool, error) {
	return newClientPool(cfg, genAddr, protoFactory, middlewares...)
}

func newClientPool(
	cfg ClientPoolConfig,
	genAddr AddressGenerator,
	proto thrift.TProtocolFactory,
	middlewares ...thrift.ClientMiddleware,
) (*clientPool, error) {
	tags := cfg.MetricsTags.AsStatsdTags()
	opener := func() (clientpool.Client, error) {
		return newClient(
			cfg.ConnectTimeout,
			cfg.SocketTimeout,
			cfg.MaxConnectionAge,
			genAddr,
			proto,
		)
	}
	pool, err := clientpool.NewChannelPool(
		cfg.InitialConnections,
		cfg.MaxConnections,
		opener,
	)
	if err != nil {
		if cfg.InitialConnectionsFallback {
			// do the InitialConnectionsFallback
			var fallbackErr error
			pool, fallbackErr = clientpool.NewChannelPool(
				0, // initialClients
				cfg.MaxConnections,
				opener,
			)
			if fallbackErr == nil {
				cfg.InitialConnectionsFallbackLogger.Log(
					context.Background(),
					"thriftbp: error initializing thrift clientpool but fallback to 0 initial connections worked. Original error: "+err.Error(),
				)
				err = nil
			} else {
				var batch errorsbp.Batch
				batch.Add(err)
				batch.Add(fallbackErr)
				err = batch.Compile()
			}
		}
		if err != nil {
			return nil, fmt.Errorf("thriftbp: error initializing thrift clientpool: %w", err)
		}
	}
	if cfg.ReportPoolStats {
		go reportPoolStats(
			metricsbp.M.Ctx(),
			cfg.ServiceSlug,
			pool,
			cfg.PoolGaugeInterval,
			tags,
		)
	}

	// create the base clientPool, this is not ready for use.
	pooledClient := &clientPool{
		Pool: pool,

		poolExhaustedCounter: metricsbp.M.Counter(
			cfg.ServiceSlug + ".pool-exhausted",
		).With(tags...),
		releaseErrorCounter: metricsbp.M.Counter(
			cfg.ServiceSlug + ".pool-release-error",
		).With(tags...),
	}
	// finish setting up the clientPool by wrapping the inner "Call" with the
	// given middleware.
	//
	// pooledClient is now ready for use.
	pooledClient.wrapCalls(middlewares...)
	return pooledClient, nil
}

func newClient(
	connectTimeout time.Duration,
	socketTimeout time.Duration,
	maxConnectionAge time.Duration,
	genAddr AddressGenerator,
	protoFactory thrift.TProtocolFactory,
) (*ttlClient, error) {
	addr, err := genAddr()
	if err != nil {
		return nil, fmt.Errorf("thriftbp: error getting next address for new Thrift client: %w", err)
	}

	trans, err := thrift.NewTSocketTimeout(addr, connectTimeout, socketTimeout)
	if err != nil {
		return nil, fmt.Errorf("thriftbp: error building TSocket for new Thrift client: %w", err)
	}

	err = trans.Open()
	if err != nil {
		return nil, fmt.Errorf("thriftbp: error opening TSocket for new Thrift client: %w", err)
	}

	var client thrift.TClient
	client = thrift.NewTStandardClient(
		protoFactory.GetProtocol(trans),
		protoFactory.GetProtocol(trans),
	)
	return newTTLClient(trans, client, maxConnectionAge), nil
}

func reportPoolStats(ctx context.Context, prefix string, pool clientpool.Pool, tickerDuration time.Duration, tags []string) {
	activeGauge := metricsbp.M.RuntimeGauge(prefix + ".pool-active-connections").With(tags...)
	allocatedGauge := metricsbp.M.RuntimeGauge(prefix + ".pool-allocated-clients").With(tags...)

	if tickerDuration <= 0 {
		tickerDuration = DefaultPoolGaugeInterval
	}
	ticker := time.NewTicker(tickerDuration)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			activeGauge.Set(float64(pool.NumActiveClients()))
			allocatedGauge.Set(float64(pool.NumAllocated()))
		}
	}
}

type clientPool struct {
	clientpool.Pool

	poolExhaustedCounter metrics.Counter
	releaseErrorCounter  metrics.Counter

	wrappedClient thrift.TClient
}

func (p *clientPool) Call(ctx context.Context, method string, args, result thrift.TStruct) (err error) {
	// A clientPool needs to be set up properly before it can be used,
	// specifically use p.wrapCalls to set up p.wrappedClient before using it.
	//
	// newClientPool already takes care of this for you.
	return p.wrappedClient.Call(ctx, method, args, result)
}

// wrapCalls wraps p.pooledCall in the given middleware and sets p.wrappedClient
// to the resulting thrift.TClient.
//
// This must be called before the clientPool can be used, but newClientPool
// already takes care of this for you.
func (p *clientPool) wrapCalls(middlewares ...thrift.ClientMiddleware) {
	p.wrappedClient = thrift.WrapClient(thrift.WrappedTClient{
		Wrapped: func(ctx context.Context, method string, args, result thrift.TStruct) error {
			return p.pooledCall(ctx, method, args, result)
		},
	}, middlewares...)
}

// pooledCall gets a Client from the inner clientpool.Pool and "Calls" it,
// returning the result and releasing the client back to the pool afterwards.
//
// This is not called directly, but is rather the inner "Call" wrapped by
// wrapCalls, so it runs after all of the middleware.
func (p *clientPool) pooledCall(ctx context.Context, method string, args, result thrift.TStruct) (err error) {
	client, err := p.getClient()
	if err != nil {
		return PoolError{Cause: err}
	}
	defer func() {
		if shouldCloseConnection(err) {
			if e := client.Close(); e != nil {
				log.Errorw("Failed to close client", "origErr", err, "closeErr", e)
			}
		}
		p.releaseClient(client)
	}()

	return client.Call(ctx, method, args, result)
}

func (p *clientPool) getClient() (Client, error) {
	c, err := p.Pool.Get()
	if err != nil {
		if errors.Is(err, clientpool.ErrExhausted) {
			p.poolExhaustedCounter.Add(1)
		}
		log.Errorw("Failed to get client from pool", "err", err)
		return nil, err
	}
	return c.(Client), nil
}

func (p *clientPool) releaseClient(c Client) {
	if err := p.Pool.Release(c); err != nil {
		log.Errorw("Failed to release client back to pool", "err", err)
		p.releaseErrorCounter.Add(1)
	}
}

func shouldCloseConnection(err error) bool {
	if err == nil {
		return false
	}
	// We should avoid reusing the client if it hits a network error.
	// We should also actively close the connection if it's a timeout,
	// as this helps the server side to abandon the request early.
	return errors.As(err, new(net.Error)) ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded)
}
