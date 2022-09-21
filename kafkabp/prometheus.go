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

var (
	rebalanceLabels = []string{
		successLabel,
	}

	rebalanceCounter = promauto.With(prometheusbpint.GlobalRegistry).NewCounterVec(prometheus.CounterOpts{
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

	consumerTimer = promauto.With(prometheusbpint.GlobalRegistry).NewHistogramVec(prometheus.HistogramOpts{
		Namespace: promNamespace,
		Subsystem: subsystemConsumer,
		Name:      "duration_seconds",
		Help:      "The time took for a non-group consumer to consume a single kafka message",
		Buckets:   prometheusbp.KafkaBuckets,
	}, timerLabels)

	groupConsumerTimer = promauto.With(prometheusbpint.GlobalRegistry).NewHistogramVec(prometheus.HistogramOpts{
		Namespace: promNamespace,
		Subsystem: subsystemGroupConsumer,
		Name:      "duration_seconds",
		Help:      "The time took for a group consumer to consume a single kafka message",
		Buckets:   prometheusbp.KafkaBuckets,
	}, timerLabels)
)

var (
	awsRackFailure = promauto.With(prometheusbpint.GlobalRegistry).NewCounter(prometheus.CounterOpts{
		Namespace: promNamespace,
		Name:      "aws_rack_id_failure_total",
		Help:      "Total failures of getting rack id from AWS endpoint",
	})

	httpRackFailure = promauto.With(prometheusbpint.GlobalRegistry).NewCounter(prometheus.CounterOpts{
		Namespace: promNamespace,
		Name:      "http_rack_id_failure_total",
		Help:      "Total failures of getting rack id from http endpoint",
	})
)
