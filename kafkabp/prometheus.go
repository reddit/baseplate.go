package kafkabp

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/reddit/baseplate.go/internal/prometheusbpint"
	"github.com/reddit/baseplate.go/prometheusbp"
)

const (
	promNamespace = "kafkabp"

	subsystemConsumer      = "consumer"
	subsystemGroupConsumer = "group_consumer"

	successLabel = "kafka_success"
	topicLabel   = "kafka_topic"
)

// TODO: Remove after next release (v0.9.12)
var (
	rebalanceLabels = []string{
		successLabel,
	}

	rebalanceCounter = promauto.With(prometheusbpint.GlobalRegistry).NewCounterVec(prometheus.CounterOpts{
		Namespace: promNamespace,
		Subsystem: subsystemConsumer,
		Name:      "rebalance_total",
		Help:      "Deprecated: use kafkabp_consumer_rebalances_total and kafkabp_consumer_rebalance_failures_total instead",
	}, rebalanceLabels)
)

var (
	rebalanceTotalCounter = promauto.With(prometheusbpint.GlobalRegistry).NewCounter(prometheus.CounterOpts{
		Namespace: promNamespace,
		Subsystem: subsystemConsumer,
		Name:      "rebalances_total",
		Help:      "The number of times consumer rebalance happened",
	})
	rebalanceFailureCounter = promauto.With(prometheusbpint.GlobalRegistry).NewCounter(prometheus.CounterOpts{
		Namespace: promNamespace,
		Subsystem: subsystemConsumer,
		Name:      "rebalance_failures_total",
		Help:      "The number of times consumer rebalance failed",
	})
)

var (
	timerLabels = []string{
		topicLabel,
	}

	consumerTimer = promauto.With(prometheusbpint.GlobalRegistry).NewHistogramVec(prometheus.HistogramOpts{
		Namespace: promNamespace,
		Subsystem: subsystemConsumer,
		Name:      "duration_seconds",
		Help:      "The time took for a non-group consumer to consume a single kafka message",
		Buckets:   prometheusbp.DefaultLatencyBuckets,
	}, timerLabels)

	groupConsumerTimer = promauto.With(prometheusbpint.GlobalRegistry).NewHistogramVec(prometheus.HistogramOpts{
		Namespace: promNamespace,
		Subsystem: subsystemGroupConsumer,
		Name:      "duration_seconds",
		Help:      "The time took for a group consumer to consume a single kafka message",
		Buckets:   prometheusbp.DefaultLatencyBuckets,
	}, timerLabels)
)

var (
	awsRackFailure = promauto.With(prometheusbpint.GlobalRegistry).NewCounter(prometheus.CounterOpts{
		Namespace: promNamespace,
		Name:      "aws_rack_id_failures_total",
		Help:      "Total failures of getting rack id from AWS endpoint",
	})

	httpRackFailure = promauto.With(prometheusbpint.GlobalRegistry).NewCounter(prometheus.CounterOpts{
		Namespace: promNamespace,
		Name:      "http_rack_id_failures_total",
		Help:      "Total failures of getting rack id from http endpoint",
	})
)

var (
	kafkaVersionGauge = promauto.With(prometheusbpint.GlobalRegistry).NewGaugeVec(prometheus.GaugeOpts{
		Name: "kafkabp_consumer_configured_version",
		Help: "The kafka version in kafkabp consumer config",
	}, []string{"kafka_version"})
)
