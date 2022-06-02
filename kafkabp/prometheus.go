package kafkabp

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	promNamespace = "kafkabp"

	subsystemConsumer      = "consumer"
	subsystemGroupConsumer = "group_consumer"

	successLabel = "kafka_success"
	topicLabel   = "kafka_topic"
)

var (
	rebalanceLabels = []string{
		successLabel,
	}

	rebalanceCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: promNamespace,
		Subsystem: subsystemConsumer,
		Name:      "rebalance_total",
		Help:      "The number of times consumer rebalance happened",
	}, rebalanceLabels)
)

var (
	timerLabels = []string{
		topicLabel,
	}

	consumerTimer = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: promNamespace,
		Subsystem: subsystemConsumer,
		Name:      "duration_seconds",
		Help:      "The time took for a non-group consumer to consume a single kafka message",
		Buckets:   prometheus.ExponentialBucketsRange(1e-4, 10, 10), // 100us - 10s
	}, timerLabels)

	groupConsumerTimer = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: promNamespace,
		Subsystem: subsystemGroupConsumer,
		Name:      "duration_seconds",
		Help:      "The time took for a group consumer to consume a single kafka message",
		Buckets:   prometheus.ExponentialBucketsRange(1e-4, 10, 10), // 100us - 10s
	}, timerLabels)
)

var (
	awsRackFailure = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: promNamespace,
		Name:      "aws_rack_id_failure_total",
		Help:      "Total failures of getting rack id from AWS endpoint",
	})

	httpRackFailure = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: promNamespace,
		Name:      "http_rack_id_failure_total",
		Help:      "Total failures of getting rack id from http endpoint",
	})
)
