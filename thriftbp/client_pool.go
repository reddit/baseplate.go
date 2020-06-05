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

	"github.com/reddit/baseplate.go/clientpool"
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
	return "thriftpb: error getting a client from the pool. " + err.Cause.Error()
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

	// SocketTimeout is the timeout on the underling thrift.TSocket.
	SocketTimeout time.Duration

	// Any labels that should be applied to metrics logged by the ClientPool.
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
			ServiceSlug:  cfg.ServiceSlug,
			RetryOptions: cfg.DefaultRetryOptions,
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
	labels := cfg.MetricsTags.AsStatsdTags()
	pool, err := clientpool.NewChannelPool(
		cfg.InitialConnections,
		cfg.MaxConnections,
		func() (clientpool.Client, error) {
			return newClient(
				cfg.SocketTimeout,
				cfg.MaxConnectionAge,
				genAddr,
				proto,
			)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("thriftbp: error initializing thrift clientpool. %w", err)
	}
	if cfg.ReportPoolStats {
		go reportPoolStats(
			metricsbp.M.Ctx(),
			cfg.ServiceSlug,
			pool,
			cfg.PoolGaugeInterval,
			labels,
		)
	}

	// create the base clientPool, this is not ready for use.
	pooledClient := &clientPool{
		Pool: pool,

		poolExhaustedCounter: metricsbp.M.Counter(
			cfg.ServiceSlug + ".pool-exhausted",
		).With(labels...),
		releaseErrorCounter: metricsbp.M.Counter(
			cfg.ServiceSlug + ".pool-release-error",
		).With(labels...),
	}
	// finish setting up the clientPool by wrapping the inner "Call" with the
	// given middleware.
	//
	// pooledClient is now ready for use.
	pooledClient.wrapCalls(middlewares...)
	return pooledClient, nil
}

func newClient(
	socketTimeout time.Duration,
	maxConnectionAge time.Duration,
	genAddr AddressGenerator,
	protoFactory thrift.TProtocolFactory,
) (*ttlClient, error) {
	addr, err := genAddr()
	if err != nil {
		return nil, fmt.Errorf("thriftbp: error getting next address for new Thrift client. %w", err)
	}

	trans, err := thrift.NewTSocketTimeout(addr, socketTimeout, socketTimeout)
	if err != nil {
		return nil, fmt.Errorf("thriftbp: error building TSocket for new Thrift client. %w", err)
	}

	err = trans.Open()
	if err != nil {
		return nil, fmt.Errorf("thriftbp: error opening TSocket for new Thrift client. %w", err)
	}

	var client thrift.TClient
	client = thrift.NewTStandardClient(
		protoFactory.GetProtocol(trans),
		protoFactory.GetProtocol(trans),
	)
	return newTTLClient(trans, client, maxConnectionAge), nil
}

func reportPoolStats(ctx context.Context, prefix string, pool clientpool.Pool, tickerDuration time.Duration, labels []string) {
	activeGauge := metricsbp.M.Gauge(prefix + ".pool-active-connections").With(labels...)
	allocatedGauge := metricsbp.M.Gauge(prefix + ".pool-allocated-clients").With(labels...)
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
		if err != nil && errors.As(err, new(net.Error)) {
			// Close the client to avoid reusing it if it's a network error.
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
