package redisprom

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/reddit/baseplate.go/internalv2compat"
	"github.com/reddit/baseplate.go/prometheusbp"
)

const (
	ClientNameLabel = "redis_client_name" // The service-provided name for the client to identify the backend for redis host(s). MUST be user specified, MAY be blank if not specified.
	DatabaseLabel   = "redis_database"    // Number of the Redis database to which the client is connecting. MAY be blank if not specified.
	SuccessLabel    = "redis_success"     // MUST BE false if the request to Redis raises an exception OR if an error was returned, otherwise true.
	TypeLabel       = "redis_type"        // MUST BE one of standalone, cluster, sentinel, identifies the backend's configuration for responding to the redis request
	DeploymentLabel = "redis_deployment"  // MUST BE one of reddit, elasticache, identifies the provider of the redis backend (not the explicit address)
	CommandLabel    = "redis_command"     // SHALL reflect to Redis command being executed for the request (ie SET)
)

var (
	LatencySeconds = promauto.With(internalv2compat.GlobalRegistry).NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "redis_client_latency_seconds",
			Help:    "latency histogram",
			Buckets: prometheusbp.DefaultBuckets,
		},
		[]string{ClientNameLabel, DatabaseLabel, TypeLabel, DeploymentLabel, CommandLabel, SuccessLabel},
	)
	ActiveRequests = promauto.With(internalv2compat.GlobalRegistry).NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "redis_client_active_requests",
			Help: "total requests that are in-flight",
		},
		[]string{ClientNameLabel, DatabaseLabel, TypeLabel, DeploymentLabel, CommandLabel},
	)
	RequestsTotal = promauto.With(internalv2compat.GlobalRegistry).NewCounterVec(
		prometheus.CounterOpts{
			Name: "redis_client_requests_total",
			Help: "total request counter",
		},
		[]string{ClientNameLabel, DatabaseLabel, TypeLabel, DeploymentLabel, CommandLabel, SuccessLabel},
	)
)
