package thriftbp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
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
	MetricsLabels metricsbp.Labels

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
	defaults := BaseplateDefaultClientMiddlewares()
	wrappers := make([]thrift.ClientMiddleware, 0, len(defaults)+len(middlewares))
	wrappers = append(wrappers, defaults...)
	wrappers = append(wrappers, middlewares...)
	return NewCustomClientPool(
		cfg,
		SingleAddressGenerator(cfg.Addr),
		thrift.NewTHeaderProtocolFactory(),
		wrappers...,
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
	labels := cfg.MetricsLabels.AsStatsdLabels()
	pool, err := clientpool.NewChannelPool(
		cfg.InitialConnections,
		cfg.MaxConnections,
		func() (clientpool.Client, error) {
			return newClient(
				cfg.SocketTimeout,
				cfg.MaxConnectionAge,
				genAddr,
				proto,
				middlewares...,
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
	return &clientPool{
		Pool: pool,

		poolExhaustedCounter: metricsbp.M.Counter(
			cfg.ServiceSlug + ".pool-exhausted",
		).With(labels...),
		releaseErrorCounter: metricsbp.M.Counter(
			cfg.ServiceSlug + ".pool-release-error",
		).With(labels...),
	}, nil
}

func newClient(
	socketTimeout time.Duration,
	maxConnectionAge time.Duration,
	genAddr AddressGenerator,
	protoFactory thrift.TProtocolFactory,
	middlewares ...thrift.ClientMiddleware,
) (*ttlClient, error) {
	addr, err := genAddr()
	if err != nil {
		return nil, fmt.Errorf("thriftbp: error getting next address for new Thrift client. %w", err)
	}

	trans, err := thrift.NewTSocketTimeout(addr, socketTimeout, socketTimeout)
	if err != nil {
		return nil, fmt.Errorf("thriftbp: error building TSocket new Thrift client. %w", err)
	}

	err = trans.Open()
	if err != nil {
		return nil, fmt.Errorf("thriftbp: error opening TSocket new Thrift client. %w", err)
	}

	var client thrift.TClient
	client = thrift.NewTStandardClient(
		protoFactory.GetProtocol(trans),
		protoFactory.GetProtocol(trans),
	)
	client = thrift.WrapClient(client, middlewares...)
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
}

func (p *clientPool) Call(ctx context.Context, method string, args, result thrift.TStruct) error {
	client, err := p.getClient()
	if err != nil {
		return PoolError{Cause: err}
	}
	defer p.releaseClient(client)

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
