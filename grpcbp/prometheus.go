package grpcbp

import (
	"github.com/reddit/baseplate.go/prometheusbp"
)

const (
	serviceLabel = "grpc_service"
	methodLabel  = "grpc_method"
	typeLabel    = "grpc_type"
	successLabel = "grpc_success"
	codeLabel    = "grpc_code"
	slugLabel    = "grpc_slug"
)

const (
	unary        = "unary"
	clientStream = "client_stream"
	serverStream = "server_stream"
)

const (
	serverMetricPrefix = "grpc_server"
	clientMetricPrefix = "grpc_client"
)

var (
	serverLabels = []string{
		serviceLabel,
		methodLabel,
		typeLabel,
		successLabel,
		codeLabel,
	}

	serverActiveRequestsLabels = []string{
		serviceLabel,
		methodLabel,
	}

	serverLatencyDistribution = prometheusbp.NewLatencyDistribution(serverMetricPrefix, serverLabels)
	serverRPCStatusCounter    = prometheusbp.NewRPCRequest(serverMetricPrefix, serverLabels)
	serverActiveRequests      = prometheusbp.NewActiveRequest(serverMetricPrefix, serverActiveRequestsLabels)
)

var (
	clientLabels = []string{
		serviceLabel,
		methodLabel,
		typeLabel,
		successLabel,
		codeLabel,
		slugLabel,
	}

	clientActiveRequestsLabels = []string{
		serviceLabel,
		methodLabel,
		slugLabel,
	}

	clientLatencyDistribution = prometheusbp.NewLatencyDistribution(clientMetricPrefix, clientLabels)
	clientRPCStatusCounter    = prometheusbp.NewRPCRequest(clientMetricPrefix, clientLabels)
	clientActiveRequests      = prometheusbp.NewActiveRequest(clientMetricPrefix, clientActiveRequestsLabels)
)
