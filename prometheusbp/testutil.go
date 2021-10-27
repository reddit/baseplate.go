package prometheusbp

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

type TestProm struct {
	ServerLatencyDistribution *prometheus.HistogramVec
	ServerRPCStatusCounter    *prometheus.CounterVec
	ServerActiveRequests      *prometheus.GaugeVec
	ClientLatencyDistribution *prometheus.HistogramVec
	ClientRPCStatusCounter    *prometheus.CounterVec
	ClientActiveRequests      *prometheus.GaugeVec
}

func (tp *TestProm) Reset() {
	tp.ServerLatencyDistribution.Reset()
	tp.ServerRPCStatusCounter.Reset()
	tp.ServerActiveRequests.Reset()
	tp.ClientLatencyDistribution.Reset()
	tp.ClientRPCStatusCounter.Reset()
	tp.ClientActiveRequests.Reset()
}

func (tp *TestProm) TestServerMetrics(labelValues, requestLabelValues []string) error {
	latencyMetricCount := testutil.CollectAndCount(tp.ServerLatencyDistribution)
	if latencyMetricCount != 1 {
		return fmt.Errorf(fmt.Sprintf("server latencyMetricCount wanted %v, got %v", 1, latencyMetricCount))
	}

	rpcMetricValue := testutil.ToFloat64(tp.ServerRPCStatusCounter.WithLabelValues(labelValues...))
	if rpcMetricValue != 1 {
		return fmt.Errorf(fmt.Sprintf("server rpcMetricValue wanted %v, got %v", 1, rpcMetricValue))
	}
	rpcMetricCount := testutil.CollectAndCount(tp.ServerRPCStatusCounter)
	if rpcMetricCount != 1 {
		return fmt.Errorf(fmt.Sprintf("server rpcMetricCount wanted %v, got %v", 1, rpcMetricCount))
	}

	requestMetricValue := testutil.ToFloat64(tp.ServerActiveRequests.WithLabelValues(requestLabelValues...))
	if requestMetricValue != 0 {
		return fmt.Errorf(fmt.Sprintf("server requestMetricValue wanted %v, got %v", 0, requestMetricValue))
	}
	requestMetricCount := testutil.CollectAndCount(tp.ServerActiveRequests)
	if requestMetricCount != 1 {
		return fmt.Errorf(fmt.Sprintf("server requestMetricCount wanted %v, got %v", 1, requestMetricCount))
	}
	return nil
}

func (tp *TestProm) TestClientMetrics(labelValues, requestLabelValues []string) error {
	latencyMetricCount := testutil.CollectAndCount(tp.ClientLatencyDistribution)
	if latencyMetricCount != 1 {
		return fmt.Errorf(fmt.Sprintf("client latencyMetricCount wanted %v, got %v", 1, latencyMetricCount))
	}

	rpcMetricValue := testutil.ToFloat64(tp.ClientRPCStatusCounter.WithLabelValues(labelValues...))
	if rpcMetricValue != 1 {
		return fmt.Errorf(fmt.Sprintf("client rpcMetricValue wanted %v, got %v", 1, rpcMetricValue))
	}
	rpcMetricCount := testutil.CollectAndCount(tp.ClientRPCStatusCounter)
	if rpcMetricCount != 1 {
		return fmt.Errorf(fmt.Sprintf("client rpcMetricCount wanted %v, got %v", 1, rpcMetricCount))
	}

	requestMetricValue := testutil.ToFloat64(tp.ClientActiveRequests.WithLabelValues(requestLabelValues...))
	if requestMetricValue != 0 {
		return fmt.Errorf(fmt.Sprintf("client requestMetricValue wanted %v, got %v", 0, requestMetricValue))
	}
	requestMetricCount := testutil.CollectAndCount(tp.ClientActiveRequests)
	if requestMetricCount != 1 {
		return fmt.Errorf(fmt.Sprintf("client requestMetricCount wanted %v, got %v", 1, requestMetricCount))
	}
	return nil
}
