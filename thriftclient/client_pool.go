package thriftclient

import (
	"context"
	"errors"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/go-kit/kit/metrics"

	"github.com/reddit/baseplate.go/clientpool"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/metricsbp"
)

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
	// ${host}:${port}
	Addr string

	// MinConnections is the minimum number of thrift connections the client
	// pool should maintain.
	MinConnections int

	// MinConnections is the maximum number of thrift connections the client
	// pool can maintain.
	MaxConnections int

	// SocketTimeout is the timeout on the underling thrift.TSocket.
	SocketTimeout time.Duration

	// ReportPoolStats signals to the ClientPool that it should report
	// statistics on the underlying clientpool.Pool in a background
	// goroutine.  If this is set to false, the reporting goroutine will
	// not be started and it will not report pool stats.
	//
	// It reports the number of active clients to a gauge named
	// "${ServiceSlug}.pool-active-connections".
	//
	// The reporting goroutine is cancelled when the global metrics client
	// context is Done.
	ReportPoolStats bool

	// PoolGaugeInterval indicates how often we should update the active
	// connections gauge when collecting pool stats.
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
	// GetClient returns a Client from the pool or creates a new one if
	// needed.
	GetClient() (Client, error)

	// ReleaseClient returns the given client to the pool.
	ReleaseClient(Client)
}

// AddressGenerator defines a function that returns the address of a thrift
// service.
type AddressGenerator func() string

// ClientFactory defines a function that builds a Client object using a
// the thrift primitives required to create a thrift.TClient.
type ClientFactory func(thrift.TTransport, thrift.TProtocolFactory) Client

// SingleAddressGenerator returns an AddressGenerator that always returns addr.
func SingleAddressGenerator(addr string) AddressGenerator {
	return func() string {
		return addr
	}
}

// MonitoredTTLClientFactory returns a ClientFactory that creates TTLClients
// with a MonitoredClient as the underlying thrift.TClient.
func MonitoredTTLClientFactory(ttl time.Duration) ClientFactory {
	return func(trans thrift.TTransport, protoFactory thrift.TProtocolFactory) Client {
		c := NewMonitoredClientFromFactory(trans, protoFactory)
		return NewTTLClient(trans, c, ttl)
	}
}

// UnmonitoredTTLClientFactory returns a ClientFactory that creates TTLClients
// with a thrift.TStandardClient as the underlying thrift.TClient.
func UnmonitoredTTLClientFactory(ttl time.Duration) ClientFactory {
	return func(trans thrift.TTransport, protoFactory thrift.TProtocolFactory) Client {
		c := thrift.NewTStandardClient(
			protoFactory.GetProtocol(trans),
			protoFactory.GetProtocol(trans),
		)
		return NewTTLClient(trans, c, ttl)
	}
}

func newClient(socketTimeout time.Duration, genAddr AddressGenerator, clientFact ClientFactory, protoFact thrift.TProtocolFactory) (Client, error) {
	trans, err := thrift.NewTSocketTimeout(genAddr(), socketTimeout)
	if err != nil {
		return nil, err
	}
	err = trans.Open()
	if err != nil {
		return nil, err
	}
	return clientFact(trans, protoFact), nil
}

func reportPoolStats(ctx context.Context, prefix string, pool clientpool.Pool, tickerDuration time.Duration) {
	gauge := metricsbp.M.Gauge(prefix + ".pool-active-connections")
	ticker := time.NewTicker(tickerDuration)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			gauge.Set(float64(pool.NumActiveClients()))
		}
	}
}

// NewTTLClientPool creates a ClientPool that can be used to create monitored
// TTLClients.
func NewTTLClientPool(ttl time.Duration, cfg ClientPoolConfig) (ClientPool, error) {
	return newClientPool(
		cfg,
		SingleAddressGenerator(cfg.Addr),
		MonitoredTTLClientFactory(ttl),
		thrift.NewTHeaderProtocolFactory(),
	)
}

// NewCustomClientPool creates a ClientPool that uses a custom AddressGenerator
// and ClientFactory.
//
// Most services will want to just use NewTTLClientPool, this has been provided
// to support services that have non-standard and/or legacy needs.
func NewCustomClientPool(cfg ClientPoolConfig, genAddr AddressGenerator, clientFact ClientFactory, protoFact thrift.TProtocolFactory) (ClientPool, error) {
	if cfg.Addr != "" {
		log.Warnf(
			"NewCustomClientPool received a non-empty cfg.Addr %q. "+
				"This will be ignored in favor of what is returned by genAddr", cfg.Addr)
	}
	return newClientPool(cfg, genAddr, clientFact, protoFact)
}

func newClientPool(cfg ClientPoolConfig, genAddr AddressGenerator, clientFact ClientFactory, protoFact thrift.TProtocolFactory) (*clientPool, error) {
	pool, err := clientpool.NewChannelPool(
		cfg.MinConnections,
		cfg.MaxConnections,
		func() (clientpool.Client, error) {
			return newClient(cfg.SocketTimeout, genAddr, clientFact, protoFact)
		},
	)
	if err != nil {
		return nil, err
	}
	if cfg.ReportPoolStats {
		go reportPoolStats(metricsbp.M.Ctx(), cfg.ServiceSlug, pool, cfg.PoolGaugeInterval)
	}
	return &clientPool{
		pool:                 pool,
		poolExhaustedCounter: metricsbp.M.Counter(cfg.ServiceSlug + ".pool-exhausted"),
		releaseErrorCounter:  metricsbp.M.Counter(cfg.ServiceSlug + ".pool-release-error"),
	}, nil
}

type clientPool struct {
	pool                 clientpool.Pool
	poolExhaustedCounter metrics.Counter
	releaseErrorCounter  metrics.Counter
}

func (p *clientPool) GetClient() (Client, error) {
	c, err := p.pool.Get()
	if err != nil {
		if errors.Is(err, clientpool.ErrExhausted) {
			p.poolExhaustedCounter.Add(1)
		}
		return nil, err
	}
	return c.(Client), nil
}

func (p *clientPool) ReleaseClient(c Client) {
	if err := p.pool.Release(c); err != nil {
		log.Errorw("Failed to release client back to pool", "err", err)
		p.releaseErrorCounter.Add(1)
	}
}
