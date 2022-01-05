package thriftbp

import (
	"github.com/apache/thrift/lib/go/thrift"
	"github.com/go-kit/kit/metrics"
	"github.com/reddit/baseplate.go/metricsbp"
)

// TODO(marco.ferrer):1/4/22 Replace with metric name that conforms to bp conventions
const meterNameTransportConnCounter = "thrift.conn.count"

type CountedTServerTransport struct {
	thrift.TServerTransport
}

func (m *CountedTServerTransport) Accept() (thrift.TTransport, error) {

	transport, err := m.TServerTransport.Accept()
	if err != nil {
		return nil, err
	}

	return NewCountedTTransport(transport), nil
}

type CountedTTransport struct {
	thrift.TTransport
	counter metrics.Counter
}

func NewCountedTTransport(transport thrift.TTransport) thrift.TTransport {
	return &CountedTTransport{
		TTransport: transport,
		counter:    metricsbp.M.Counter(meterNameTransportConnCounter),
	}
}

func (m *CountedTTransport) Close() error {
	m.counter.Add(-1)
	return m.TTransport.Close()
}

func (m *CountedTTransport) Open() error {
	err := m.TTransport.Open()
	if err == nil {
		m.counter.Add(1)
	}
	return err
}
