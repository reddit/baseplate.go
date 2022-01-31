package thriftbp

import (
	"sync"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/go-kit/kit/metrics"

	"github.com/reddit/baseplate.go/metricsbp"
)

const meterNameTransportConnCounter = "thrift.connections"

type CountedTServerTransport struct {
	thrift.TServerTransport
}

func (m *CountedTServerTransport) Accept() (thrift.TTransport, error) {
	transport, err := m.TServerTransport.Accept()
	if err != nil {
		return nil, err
	}

	return newCountedTTransport(transport), nil
}

type countedTTransport struct {
	thrift.TTransport

	gauge     metrics.Gauge
	closeOnce sync.Once
}

func newCountedTTransport(transport thrift.TTransport) thrift.TTransport {
	return &countedTTransport{
		TTransport: transport,
		gauge:      metricsbp.M.RuntimeGauge(meterNameTransportConnCounter),
	}
}

func (m *countedTTransport) Close() error {
	m.closeOnce.Do(func() {
		m.gauge.Add(-1)
	})
	return m.TTransport.Close()
}

func (m *countedTTransport) Open() error {
	if err := m.TTransport.Open(); err != nil {
		return err
	}
	m.gauge.Add(1)
	return nil
}
