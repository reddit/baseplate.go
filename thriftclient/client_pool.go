package thriftclient

import (
	"context"
	"errors"
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

// ClientPoolConfig is the configuration struct for creating a new ClientPool.
type ClientPoolConfig struct {
	// ServiceSlug is a short identifier for the thrift service you are creating
	// clients for.  The preferred convention is to take the service's name,
	// remove the 'Service' prefix, if present, and convert from camel case to
	// all lower case, hyphen separated.
	//
	// Example:
	//		AuthenticationService -> authentication
	//		ImageUploadService -> image-upload
	ServiceSlug string

	// Addr is the address of a thrift service.  Addr must be in the format
	// "${host}:${port}"
	Addr string

	// InitialConnections is the inital number of thrift connections created by
	// the client pool.
	InitialConnections int

	// MinConnections is the maximum number of thrift connections the client
	// pool can maintain.
	MaxConnections int

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

// ClientPool defines an object that can be used to manage a pool of
// Client objects.
type ClientPool interface {
	// Passthrough APIs from clientpool.Pool:
	io.Closer
	IsExhausted() bool

	// GetClient returns a Client from the pool or creates a new one if
	// needed.
	GetClient() (Client, error)

	// ReleaseClient returns the given client to the pool.
	ReleaseClient(Client)
}

// AddressGenerator defines a function that returns the address of a thrift
// service.
//
// Services should generally not have to use AddressGenerators directly,
// instead you should use NewBaseplateClientPool which uses the default
// AddressGenerator for a typical Baseplate Thrift Client.
type AddressGenerator func() (string, error)

// ClientFactory defines a function that builds a Client object using a
// the thrift primitives required to create a thrift.TClient.
//
// Services should generally not have to use ClientFactories directly,
// instead you should use NewBaseplateClientPool which uses the default
// ClientFactory for a typical Baseplate Thrift Client.
type ClientFactory func(TClientFactory, thrift.TTransport, thrift.TProtocolFactory) Client

// TClientFactory is used by ClientFactory to create the underlying TClient
// used by the Baseplate Client.
//
// Services should generally not have to use TClientFactories directly,
// instead you should use NewBaseplateClientPool which uses the default
// TClientFactory for a typical Baseplate Thrift Client.
type TClientFactory func(thrift.TTransport, thrift.TProtocolFactory) thrift.TClient

// NewBaseplateClientPool returns a standard, baseplate ClientPool.
//
// A baseplate ClientPool:
//		1. Uses a TTLClientPool with the given ttl.
//		2. Wraps the TClient objects with BaseplateDefaultMiddlewares plus any
//		   additional middlewares passed into this function.
func NewBaseplateClientPool(cfg ClientPoolConfig, ttl time.Duration, middlewares ...Middleware) (ClientPool, error) {
	defaults := BaseplateDefaultMiddlewares()
	wrappers := make([]Middleware, 0, len(defaults)+len(middlewares))
	wrappers = append(wrappers, defaults...)
	wrappers = append(wrappers, middlewares...)
	return NewCustomClientPool(
		cfg,
		SingleAddressGenerator(cfg.Addr),
		NewTTLClientFactory(ttl),
		NewWrappedTClientFactory(StandardTClientFactory, wrappers...),
		thrift.NewTHeaderProtocolFactory(),
	)
}

// NewCustomClientPool creates a ClientPool that uses a custom AddressGenerator
// and ClientFactory.
//
// Most services will want to just use NewBaseplateClientPool, this has been
// provided to support services that have non-standard and/or legacy needs.
func NewCustomClientPool(
	cfg ClientPoolConfig,
	genAddr AddressGenerator,
	clientFactory ClientFactory,
	tClientFactory TClientFactory,
	protoFactory thrift.TProtocolFactory,
) (ClientPool, error) {
	if cfg.Addr != "" {
		log.Warnw(
			"NewCustomClientPool received a non-empty cfg.Addr, "+
				"this will be ignored in favor of what is returned by genAddr",
			"addr",
			cfg.Addr,
		)
	}
	return newClientPool(cfg, genAddr, factories{
		Client:   clientFactory,
		TClient:  tClientFactory,
		Protocol: protoFactory,
	})
}

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

// NewTTLClientFactory returns a ClientFactory that creates TTLClients using the
// given TClientFactory.
//
// Services should generally not have to use NewTTLClientFactory directly,
// instead you should use NewBaseplateClientPool which uses the default
// ClientFactory for a typical Baseplate Thrift Client.
func NewTTLClientFactory(ttl time.Duration) ClientFactory {
	return func(clientFact TClientFactory, trans thrift.TTransport, protoFactory thrift.TProtocolFactory) Client {
		return NewTTLClient(trans, clientFact(trans, protoFactory), ttl)
	}
}

// StandardTClientFactory returns a standard thrift.TClient using the given
// thrift.TTransport and thrift.TProtocolFactory
//
// Services should generally not have to use StandardTClientFactory
// directly, instead you should use NewBaseplateClientPool which uses the
// default TClientFactory for a typical Baseplate Thrift Client.
func StandardTClientFactory(trans thrift.TTransport, protoFactory thrift.TProtocolFactory) thrift.TClient {
	return thrift.NewTStandardClient(
		protoFactory.GetProtocol(trans),
		protoFactory.GetProtocol(trans),
	)
}

// NewWrappedTClientFactory returns a TClientFactory that returns a standard
// thrift.TClient wrapped with the given middlewares.
//
// Services should generally not have to use NewWrappedTClientFactory
// directly, instead you should use NewBaseplateClientPool which uses the
// default TClientFactory for a typical Baseplate Thrift Client.
func NewWrappedTClientFactory(base TClientFactory, middlewares ...Middleware) TClientFactory {
	return func(trans thrift.TTransport, protoFactory thrift.TProtocolFactory) thrift.TClient {
		client := base(trans, protoFactory)
		return Wrap(client, middlewares...)
	}
}

// convenience struct for passing around the different factories needed to
// create a Client.
type factories struct {
	Client   ClientFactory
	TClient  TClientFactory
	Protocol thrift.TProtocolFactory
}

func newClientPool(cfg ClientPoolConfig, genAddr AddressGenerator, factories factories) (*clientPool, error) {
	labels := cfg.MetricsLabels.AsStatsdLabels()
	pool, err := clientpool.NewChannelPool(
		cfg.InitialConnections,
		cfg.MaxConnections,
		func() (clientpool.Client, error) {
			return newClient(cfg.SocketTimeout, genAddr, factories)
		},
	)
	if err != nil {
		return nil, err
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

func newClient(socketTimeout time.Duration, genAddr AddressGenerator, factories factories) (Client, error) {
	addr, err := genAddr()
	if err != nil {
		return nil, err
	}
	trans, err := thrift.NewTSocketTimeout(addr, socketTimeout, socketTimeout)
	if err != nil {
		return nil, err
	}
	err = trans.Open()
	if err != nil {
		return nil, err
	}
	return factories.Client(factories.TClient, trans, factories.Protocol), nil
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

func (p *clientPool) GetClient() (Client, error) {
	c, err := p.Pool.Get()
	if err != nil {
		if errors.Is(err, clientpool.ErrExhausted) {
			p.poolExhaustedCounter.Add(1)
		}
		return nil, err
	}
	return c.(Client), nil
}

func (p *clientPool) ReleaseClient(c Client) {
	if err := p.Pool.Release(c); err != nil {
		log.Errorw("Failed to release client back to pool", "err", err)
		p.releaseErrorCounter.Add(1)
	}
}
