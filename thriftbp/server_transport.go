package thriftbp

import (
	"sync"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/go-kit/kit/metrics"

	"github.com/reddit/baseplate.go/metricsbp"
)

const meterNameTransportConnCounter = "thrift.connections"

// CountedTServerTransport is a wrapper around thrift.TServerTransport that
// emits a gauge of the number of client connections.
type CountedTServerTransport struct {
	thrift.TServerTransport
}

// Accept implements thrift.TServerTransport by retruning a thrift.TTransport
// that counts the number of client connections.
func (m *CountedTServerTransport) Accept() (thrift.TTransport, error) {
	transport, err := m.TServerTransport.Accept()
	if err != nil {
		return nil, err
	}

	wrappedTransport := newCountedTTransport(transport)
	wrappedTransport.gauge.Add(1)
	serverConnectionsGauge.Inc()
	return wrappedTransport, nil
}

type countedTTransport struct {
	thrift.TTransport

	gauge     metrics.Gauge
	closeOnce sync.Once
}

func newCountedTTransport(transport thrift.TTransport) *countedTTransport {
	return &countedTTransport{
		TTransport: transport,
		gauge:      metricsbp.M.RuntimeGauge(meterNameTransportConnCounter),
	}
}

func (m *countedTTransport) Close() error {
	m.closeOnce.Do(func() {
		m.gauge.Add(-1)
		serverConnectionsGauge.Dec()
	})
	return m.TTransport.Close()
}

func (m *countedTTransport) Open() error {
	if err := m.TTransport.Open(); err != nil {
		return err
	}
	m.gauge.Add(1)
	serverConnectionsGauge.Inc()
	return nil
}
