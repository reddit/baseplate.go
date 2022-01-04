package thriftbp

import (
	"github.com/apache/thrift/lib/go/thrift"
	"github.com/reddit/baseplate.go/metricsbp"
)

// TODO(marco.ferrer):1/4/22 Replace with metric name that conforms to bp conventions
const meterNameTransportConnCounter = "thrift.conn.count"

type BaseplateWrapperTServerTransport struct {
	thrift.TServerTransport
	reportConnectionCountMetrics bool
}

func (m BaseplateWrapperTServerTransport) Accept() (thrift.TTransport, error) {

	transport, err := m.TServerTransport.Accept()
	if err != nil {
		return nil, err
	}

	if m.reportConnectionCountMetrics {
		transport = MonitoredTTransport{transport}
	}

	return transport, nil
}

type MonitoredTTransport struct {
	thrift.TTransport
}

func (m MonitoredTTransport) Close() error {
	defer metricsbp.M.
		Counter(meterNameTransportConnCounter).
		Add(-1)
	return m.TTransport.Close()
}

func (m MonitoredTTransport) Open() error {
	defer metricsbp.M.
		Counter(meterNameTransportConnCounter).
		Add(1)
	return m.TTransport.Open()
}
