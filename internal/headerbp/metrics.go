package headerbp

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/reddit/baseplate.go/internal/prometheusbpint"
)

const (
	clientMethodLabel = "client_method"
	clientNameLabel   = "client_name"
	headerNameLabel   = "header_name"
	rpcTypeLabel      = "rpc_type"
	serverMethodLabel = "server_method"
	serviceLabel      = "service_name"
)

var (
	clientHeadersRejectedTotal = promauto.With(prometheusbpint.GlobalRegistry).NewCounterVec(prometheus.CounterOpts{
		Name: "baseplate_client_headers_rejected_total",
		Help: "Total number of requests that were rejected by a client due to unapproved internal headers",
	}, []string{
		rpcTypeLabel,
		serviceLabel,
		clientNameLabel,
		clientMethodLabel,
		headerNameLabel,
	})

	clientHeadersSentTotal = promauto.With(prometheusbpint.GlobalRegistry).NewHistogramVec(prometheus.HistogramOpts{
		Name:    "baseplate_client_headers_sent_total",
		Help:    "Total number of internal headers that were automatically sent by a client",
		Buckets: []float64{1, 4, 8, 16, 32, 64, 128},
	}, []string{
		rpcTypeLabel,
		serviceLabel,
		clientNameLabel,
		clientMethodLabel,
	})

	clientHeadersSentSize = promauto.With(prometheusbpint.GlobalRegistry).NewHistogramVec(prometheus.HistogramOpts{
		Name:    "baseplate_client_headers_sent_size_total",
		Help:    "Estimated size (in bytes) of internal headers that were automatically sent by a client",
		Buckets: []float64{1, 64, 128, 256, 512, 1024, 2048, 4096},
	}, []string{
		rpcTypeLabel,
		serviceLabel,
		clientNameLabel,
		clientMethodLabel,
	})

	serverHeadersReceivedTotal = promauto.With(prometheusbpint.GlobalRegistry).NewCounterVec(prometheus.CounterOpts{
		Name: "baseplate_service_headers_received_total",
		Help: "Total number of internal headers that were automatically extracted by a server",
	}, []string{
		rpcTypeLabel,
		serviceLabel,
		serverMethodLabel,
		headerNameLabel,
	})

	serverHeadersReceivedSize = promauto.With(prometheusbpint.GlobalRegistry).NewHistogramVec(prometheus.HistogramOpts{
		Name:    "baseplate_server_headers_received_size_total",
		Help:    "Estimated size (in bytes) of internal headers that were automatically extracted by a server",
		Buckets: []float64{1, 64, 128, 256, 512, 1024, 2048, 4096},
	}, []string{
		rpcTypeLabel,
		serviceLabel,
		serverMethodLabel,
	})
)
