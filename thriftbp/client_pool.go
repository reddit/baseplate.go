package thriftbp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/avast/retry-go"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/reddit/baseplate.go/breakerbp"
	"github.com/reddit/baseplate.go/clientpool"
	"github.com/reddit/baseplate.go/ecinterface"
	"github.com/reddit/baseplate.go/errorsbp"
	"github.com/reddit/baseplate.go/internal/prometheusbpint"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/metricsbp"
)

// DefaultPoolGaugeInterval is the fallback value to be used when
// ClientPoolConfig.PoolGaugeInterval <= 0.
//
// Deprecated: Prometheus gauges are auto scraped.
const DefaultPoolGaugeInterval = time.Second * 10

// PoolError is returned by ClientPool.TClient.Call when it fails to get a
// client from its pool.
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
	ServiceSlug string `yaml:"serviceSlug"`

	// Addr is the address of a thrift service.  Addr must be in the format
	// "${host}:${port}"
	Addr string `yaml:"addr"`

	// InitialConnections is the desired inital number of thrift connections
	// created by the client pool.
	//
	// If an error occurred when we try to establish the initial N connections for
	// the pool, we log the errors on warning level,
	// then return the pool with the <N connections we already established.
	//
	// If that's unacceptable, then RequiredInitialConnections can be used to set
	// the hard requirement of minimal initial connections to be established.
	// Note that enabling this can cause cascading failures during an outage,
	// so it shall only be used for extreme circumstances.
	InitialConnections         int `yaml:"initialConnections"`
	RequiredInitialConnections int `yaml:"requiredInitialConnections"`
	// Deprecated: InitialConnectionsFallback is always true and setting it to
	// false won't do anything.
	InitialConnectionsFallback bool `yaml:"initialConnectionsFallback"`
	// Deprecated: Individual connection errors during initialization is always
	// logged via zap logger on warning level.
	InitialConnectionsFallbackLogger log.Wrapper `yaml:"initialConnectionsFallbackLogger"`

	// MaxConnections is the maximum number of thrift connections the client
	// pool can maintain.
	MaxConnections int `yaml:"maxConnections"`

	// MinConnections is the minimum number of thrift connections (idle+active)
	// that the client pool will try to maintain via a background worker.
	//
	// If this value is 0 or negative, the background worker will not be started.
	MinConnections int `yaml:"MinConnections"`

	// BackgroundTaskInterval is the interval that the connection pool will check
	// and try to ensure that there are MinConnections in the pool.
	//
	// If this is not set, the default duration is 5 seconds.
	BackgroundTaskInterval time.Duration `yaml:"BackgroundTaskInterval"`

	// MaxConnectionAge is the maximum duration that a pooled connection will be
	// kept before closing in favor of a new one.
	//
	// If this is not set, the default duration is 5 minutes
	// (see DefaultMaxConnectionAge).
	//
	// To disable this and keep connections in the pool indefinetly, set this to
	// a negative value.
	//
	// MaxConnectionAgeJitter is the ratio of random jitter +/- on top of
	// MaxConnectionAge. Default to 10% (see DefaultMaxConnectionAgeJitter).
	// For example, when MaxConnectionAge is 5min and MaxConnectionAgeJitter is
	// 10%, the TTL of the clients would be in range of (4:30, 5:30).
	//
	// When this is enabled, there will be one additional goroutine per connection
	// in the pool to do background housekeeping (to replace the expired
	// connections). We emit thriftbp_ttlclient_connection_housekeeping_total
	// counter with thrift_success tag to provide observalibility into the
	// background housekeeping.
	//
	// Due to a Go runtime bug [1], if you use a very small MaxConnectionAge or a
	// jitter very close to 1, the background housekeeping could cause excessive
	// CPU overhead.
	//
	// [1]: https://github.com/golang/go/issues/27707
	MaxConnectionAge       time.Duration `yaml:"maxConnectionAge"`
	MaxConnectionAgeJitter *float64      `yaml:"maxConnectionAgeJitter"`

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
	ConnectTimeout time.Duration `yaml:"connectTimeout"`
	SocketTimeout  time.Duration `yaml:"socketTimeout"`

	// Any tags that should be applied to metrics logged by the ClientPool.
	// This includes the optional pool stats.
	//
	// Deprecated: We no longer emit any statsd metrics so this has no effect.
	MetricsTags metricsbp.Tags `yaml:"metricsTags"`

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
	//
	// Deprecated: The statsd metrics are deprecated and the prometheus metrics
	// are always reported.
	ReportPoolStats bool `yaml:"reportPoolStats"`

	// PoolGaugeInterval indicates how often we should update the active
	// connections gauge when collecting pool stats.
	//
	// When PoolGaugeInterval <= 0 and ReportPoolStats is true,
	// DefaultPoolGaugeInterval will be used instead.
	//
	// Deprecated: Not used any more. Prometheus gauges are auto scraped.
	PoolGaugeInterval time.Duration `yaml:"poolGaugeInterval"`

	// Suppress some of the errors returned by the server before sending them to
	// the client span.
	//
	// See MonitorClientArgs.ErrorSpanSuppressor for more details.
	//
	// This is optional. If it's not set IDLExceptionSuppressor will be used.
	ErrorSpanSuppressor errorsbp.Suppressor

	// When BreakerConfig is non-nil,
	// a breakerbp.FailureRatioBreaker will be created for the pool,
	// and its middleware will be set for the pool.
	BreakerConfig *breakerbp.Config `yaml:"breakerConfig"`

	// The edge context implementation. Optional.
	//
	// If it's not set, the global one from ecinterface.Get will be used instead.
	EdgeContextImpl ecinterface.Interface

	// The name for the server to identify this client,
	// via the "User-Agent" (HeaderUserAgent) THeader.
	//
	// Optional. If this is empty, no "User-Agent" header will be sent.
	ClientName string `yaml:"clientName"`
}

// Validate checks ClientPoolConfig for any missing or erroneous values.
//
// If this is the configuration for a baseplate service
// BaseplateClientPoolConfig(c).Validate should be used instead.
func (c ClientPoolConfig) Validate() error {
	if c.InitialConnections > c.MaxConnections {
		return ErrConfigInvalidConnections
	}
	if c.MinConnections > c.MaxConnections {
		return ErrConfigInvalidMinConnections
	}
	return nil
}

var tHeaderProtocolCompact = thrift.THeaderProtocolIDPtrMust(thrift.THeaderProtocolCompact)

// ToTConfiguration generates *thrift.TConfiguration from this config.
//
// Note that it always set THeaderProtocolID to thrift.THeaderProtocolCompact,
// even though that's not part of the ClientPoolConfig.
// To override this behavior, change the value from the returned TConfiguration.
func (c ClientPoolConfig) ToTConfiguration() *thrift.TConfiguration {
	return &thrift.TConfiguration{
		ConnectTimeout:    c.ConnectTimeout,
		SocketTimeout:     c.SocketTimeout,
		THeaderProtocolID: thrift.THeaderProtocolIDPtrMust(*tHeaderProtocolCompact),
	}
}

// BaseplateClientPoolConfig provides a more concrete Validate method tailored
// to validating baseplate service confgiurations.
type BaseplateClientPoolConfig ClientPoolConfig

// Validate checks the BaseplateClientPoolConfig for any missing or erroneous values.
//
// This method is designated to be used when passing a configuration to
// NewBaseplateClientPool, for NewCustomClientPool other constraints apply.
func (c BaseplateClientPoolConfig) Validate() error {
	var errs []error
	if c.ServiceSlug == "" {
		errs = append(errs, ErrConfigMissingServiceSlug)
	}
	if c.Addr == "" {
		errs = append(errs, ErrConfigMissingAddr)
	}
	if c.InitialConnections > c.MaxConnections {
		errs = append(errs, ErrConfigInvalidConnections)
	}
	if c.MinConnections > c.MaxConnections {
		errs = append(errs, ErrConfigInvalidMinConnections)
	}
	return errors.Join(errs...)
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
//
// A ClientPool is safe to be shared across different goroutines,
// but a concrete thrift client created on top of it is not.
// A concrete thrift client is the one you created from thrift compiled go
// code, for example baseplate.NewBaseplateServiceV2Client(pool).
// You need to create a concrete thrift client for each of your goroutines,
// but they can share the same ClientPool underneath.
type ClientPool interface {
	// The returned TClient implements TClient by grabbing a Client from its pool
	// and releasing that Client after its Call method completes.
	//
	// A typical example of how to use the pool is like this:
	//
	//     // service.NewServiceClient comes from thrift compiled go code.
	//     // client you got here should be treated as disposable and never be
	//     // shared between goroutines.
	//     client := service.NewServiceClient(pool.TClient())
	//     resp, err := client.MyEndpoint(ctx, req)
	//
	// Or you can create a "client factory" for the service you want to call:
	//
	//    type ServiceClientFactory struct {
	//        pool thriftbp.ClientPool
	//    }
	//
	//    // service.Service and service.NewServiceClient are from thrift compiled
	//    // go code.
	//    func (f ServiceClientFactory) Client service.Service {
	//        return service.NewServiceClient(f.pool.TClient())
	//    }
	//
	//    client := factory.Client()
	//    resp, err := client.MyEndpoint(ctx, req)
	//
	// If the call fails to get a client from the pool, it will return PoolError.
	// You can check the error returned using:
	//
	//     var poolErr thriftbp.PoolError
	//     if errors.As(err, &poolErr) {
	//       // It's unable to get a client from the pool
	//     } else {
	//       // It's error from the actual thrift call
	//     }
	//
	// If the error is not of type PoolError that means it's returned by the call
	// from the actual client.
	//
	// If the call fails to release the client back to the pool,
	// it will log the error on error level but not return it to the caller.
	// It also increases thriftbp_client_pool_release_errors_total counter.
	TClient() thrift.TClient

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

// NewBaseplateClientPool calls NewBaseplateClientPoolWithContext with
// background context. It should not be used with RequiredInitialConnections > 0.
func NewBaseplateClientPool(cfg ClientPoolConfig, middlewares ...thrift.ClientMiddleware) (ClientPool, error) {
	return NewBaseplateClientPoolWithContext(context.Background(), cfg, middlewares...)
}

// NewBaseplateClientPoolWithContext returns a standard ClientPool wrapped with
// the BaseplateDefaultClientMiddlewares plus any additional client middlewares
// passed into this function.
//
// It always uses SingleAddressGenerator with the server address configured in
// cfg, and THeader+TCompact as the protocol factory.
//
// If you have RequiredInitialConnections > 0, ctx passed in controls the
// timeout of retries to hit required initial connections. Having a ctx without
// timeout with a downed upstream could cause this function to be blocked
// forever.
func NewBaseplateClientPoolWithContext(ctx context.Context, cfg ClientPoolConfig, middlewares ...thrift.ClientMiddleware) (ClientPool, error) {
	err := BaseplateClientPoolConfig(cfg).Validate()
	if err != nil {
		return nil, fmt.Errorf("thriftbp.NewBaseplateClientPool: %w", err)
	}
	defaults := BaseplateDefaultClientMiddlewares(
		DefaultClientMiddlewareArgs{
			EdgeContextImpl:     cfg.EdgeContextImpl,
			ServiceSlug:         cfg.ServiceSlug,
			RetryOptions:        cfg.DefaultRetryOptions,
			ErrorSpanSuppressor: cfg.ErrorSpanSuppressor,
			BreakerConfig:       cfg.BreakerConfig,
			ClientName:          cfg.ClientName,
		},
	)
	middlewares = append(middlewares, defaults...)
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("thriftbp.NewCustomClientPool: %w", err)
	}
	return newClientPool(
		ctx,
		cfg,
		SingleAddressGenerator(cfg.Addr),
		thrift.NewTHeaderProtocolFactoryConf(cfg.ToTConfiguration()),
		middlewares...,
	)
}

// NewCustomClientPool calls NewCustomClientPoolWithContext with background
// context. It should not be used with RequiredInitialConnections > 0.
func NewCustomClientPool(
	cfg ClientPoolConfig,
	genAddr AddressGenerator,
	protoFactory thrift.TProtocolFactory,
	middlewares ...thrift.ClientMiddleware,
) (ClientPool, error) {
	return NewCustomClientPoolWithContext(context.Background(), cfg, genAddr, protoFactory, middlewares...)
}

// NewCustomClientPoolWithContext creates a ClientPool that uses a custom
// AddressGenerator and TProtocolFactory wrapped with the given middleware.
//
// Most services will want to just use NewBaseplateClientPoolWithContext, this
// has been provided to support services that have non-standard and/or legacy
// needs.
//
// If you have RequiredInitialConnections > 0, ctx passed in controls the
// timeout of retries to hit required initial connections. Having a ctx without
// timeout with a downed upstream could cause this function to be blocked
// forever.
func NewCustomClientPoolWithContext(
	ctx context.Context,
	cfg ClientPoolConfig,
	genAddr AddressGenerator,
	protoFactory thrift.TProtocolFactory,
	middlewares ...thrift.ClientMiddleware,
) (ClientPool, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("thriftbp.NewCustomClientPool: %w", err)
	}
	return newClientPool(ctx, cfg, genAddr, protoFactory, middlewares...)
}

func newClientPool(
	ctx context.Context,
	cfg ClientPoolConfig,
	genAddr AddressGenerator,
	proto thrift.TProtocolFactory,
	middlewares ...thrift.ClientMiddleware,
) (*clientPool, error) {
	clientPoolMaxSizeGauge.With(prometheus.Labels{
		"thrift_pool": cfg.ServiceSlug,
	}).Set(float64(cfg.MaxConnections))
	tConfig := cfg.ToTConfiguration()
	jitter := DefaultMaxConnectionAgeJitter
	if cfg.MaxConnectionAgeJitter != nil {
		jitter = *cfg.MaxConnectionAgeJitter
	}
	opener := func() (clientpool.Client, error) {
		// opener is only called in 2 scenarios:
		//
		// 1. fill in the initial clients when initialize a client pool
		// 2. we failed to get a open client from the pool when trying to use it,
		//    so we have to fallback to call opener to open a new one
		//
		// so this counter gives us _good enough_ data (when igoring scenario 1),
		// in combination with clientPoolGetsCounter, to understand how many client
		// calls had to do dns on hot path.
		clientPoolOpenerCounter.With(prometheus.Labels{
			"thrift_pool": cfg.ServiceSlug,
		}).Inc()

		return newClient(
			tConfig,
			cfg.ServiceSlug,
			cfg.MaxConnectionAge,
			jitter,
			genAddr,
			proto,
		)
	}

	pool, err := clientpool.NewChannelPoolWithMinClients(
		ctx,
		cfg.RequiredInitialConnections,
		cfg.InitialConnections,
		cfg.MinConnections,
		cfg.MaxConnections,
		opener,
		cfg.BackgroundTaskInterval,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"thriftbp: error initializing the required number of connections in the thrift clientpool for %q: %w",
			cfg.ServiceSlug,
			err,
		)
	}

	if err := prometheusbpint.GlobalRegistry.Register(&clientPoolGaugeExporter{
		slug: cfg.ServiceSlug,
		pool: pool,
	}); err != nil {
		// Register should never fail because clientPoolGaugeExporter.Describe is
		// a no-op, but just in case.

		return nil, fmt.Errorf(
			"thriftbp: error registering prometheus exporter for client pool %q: %w",
			cfg.ServiceSlug,
			errors.Join(
				err,
				errorsbp.Prefix("close pool", pool.Close()),
			),
		)
	}

	// create the base clientPool, this is not ready for use.
	pooledClient := &clientPool{
		Pool: pool,

		slug: cfg.ServiceSlug,
	}
	// finish setting up the clientPool by wrapping the inner "Call" with the
	// given middleware.
	//
	// pooledClient is now ready for use.
	pooledClient.wrapCalls(middlewares...)

	// Register the error prometheus counters so they can be monitored
	labels := prometheus.Labels{
		"thrift_pool": cfg.ServiceSlug,
	}
	clientPoolExhaustedCounter.With(labels)
	clientPoolClosedConnectionsCounter.With(labels)
	clientPoolReleaseErrorCounter.With(labels)

	return pooledClient, nil
}

func newClient(
	cfg *thrift.TConfiguration,
	slug string,
	maxConnectionAge time.Duration,
	maxConnectionAgeJitter float64,
	genAddr AddressGenerator,
	protoFactory thrift.TProtocolFactory,
) (*ttlClient, error) {
	return newTTLClient(func() (thrift.TClient, thrift.TTransport, error) {
		addr, err := genAddr()
		if err != nil {
			return nil, nil, fmt.Errorf("thriftbp: error getting next address for new Thrift client: %w", err)
		}

		transport := thrift.NewTSocketConf(addr, cfg)
		if err := transport.Open(); err != nil {
			return nil, nil, fmt.Errorf("thriftbp: error opening TSocket for new Thrift client: %w", err)
		}

		return thrift.NewTStandardClient(
			protoFactory.GetProtocol(transport),
			protoFactory.GetProtocol(transport),
		), transport, nil
	}, maxConnectionAge, maxConnectionAgeJitter, slug)
}

type clientPool struct {
	clientpool.Pool

	slug string

	wrappedClient thrift.TClient
}

func (p *clientPool) TClient() thrift.TClient {
	// A clientPool needs to be set up properly before it can be used,
	// specifically use p.wrapCalls to set up p.wrappedClient before using it.
	//
	// newClientPool already takes care of this for you.
	return p.wrappedClient
}

// wrapCalls wraps p.pooledCall in the given middleware and sets p.wrappedClient
// to the resulting thrift.TClient.
//
// This must be called before the clientPool can be used, but newClientPool
// already takes care of this for you.
func (p *clientPool) wrapCalls(middlewares ...thrift.ClientMiddleware) {
	p.wrappedClient = thrift.WrapClient(thrift.WrappedTClient{
		Wrapped: func(ctx context.Context, method string, args, result thrift.TStruct) (thrift.ResponseMeta, error) {
			return p.pooledCall(ctx, method, args, result)
		},
	}, middlewares...)
}

// pooledCall gets a Client from the inner clientpool.Pool and "Calls" it,
// returning the result and releasing the client back to the pool afterwards.
//
// This is not called directly, but is rather the inner "Call" wrapped by
// wrapCalls, so it runs after all of the middleware.
func (p *clientPool) pooledCall(ctx context.Context, method string, args, result thrift.TStruct) (_ thrift.ResponseMeta, err error) {
	var client Client
	client, err = p.getClient()
	if err != nil {
		return thrift.ResponseMeta{}, PoolError{Cause: err}
	}
	defer func() {
		if shouldCloseConnection(err) {
			clientPoolClosedConnectionsCounter.With(prometheus.Labels{
				"thrift_pool": p.slug,
			}).Inc()
			if e := client.Close(); e != nil {
				log.C(ctx).Errorw(
					"Failed to close client",
					"pool", p.slug,
					"origErr", err,
					"closeErr", e,
				)
			}
		}
		p.releaseClient(client)
	}()

	return client.Call(ctx, method, args, result)
}

func (p *clientPool) getClient() (_ Client, err error) {
	defer func() {
		clientPoolGetsCounter.With(prometheus.Labels{
			"thrift_pool":    p.slug,
			"thrift_success": strconv.FormatBool(err == nil),
		}).Inc()
	}()
	c, err := p.Pool.Get()
	if err != nil {
		if errors.Is(err, clientpool.ErrExhausted) {
			clientPoolExhaustedCounter.With(prometheus.Labels{
				"thrift_pool": p.slug,
			}).Inc()
		}
		log.Errorw(
			"Failed to get client from pool",
			"pool", p.slug,
			"err", err,
		)
		return nil, err
	}
	return c.(Client), nil
}

func (p *clientPool) releaseClient(c Client) {
	if err := p.Pool.Release(c); err != nil {
		log.Errorw(
			"Failed to release client back to pool",
			"pool", p.slug,
			"err", err,
		)
		clientPoolReleaseErrorCounter.With(prometheus.Labels{
			"thrift_pool": p.slug,
		}).Inc()
	}
}

func shouldCloseConnection(err error) bool {
	if err == nil {
		return false
	}
	var te thrift.TException
	if errors.As(err, &te) {
		switch te.TExceptionType() {
		case thrift.TExceptionTypeApplication, thrift.TExceptionTypeProtocol, thrift.TExceptionTypeTransport:
			// We definitely should close the connection on TTransportException.
			// We probably don't need to close the connection on TApplicationException
			// and TProtocolException, but just close them to be safe, as the
			// connection might be in some weird state when those errors happen.
			return true
		case thrift.TExceptionTypeCompiled:
			// For exceptions defined from the IDL, we definitely shouldn't close the
			// connection.
			return false
		}
	}
	// We should avoid reusing the client if it hits a network error.
	// We should also actively close the connection if it's a timeout,
	// as this helps the server side to abandon the request early.
	return errors.As(err, new(net.Error)) ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded)
}
