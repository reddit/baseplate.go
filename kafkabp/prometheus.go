package kafkabp

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/reddit/baseplate.go/internal/prometheusbpint"
	"github.com/reddit/baseplate.go/prometheusbp"
)

const (
	topicLabel = "kafka_topic"
)

var (
	rebalanceTotalCounter = promauto.With(prometheusbpint.GlobalRegistry).NewCounter(prometheus.CounterOpts{
		Name: "kafkabp_consumer_rebalances_total",
		Help: "The number of times consumer rebalance happened",
	})
	rebalanceFailureCounter = promauto.With(prometheusbpint.GlobalRegistry).NewCounter(prometheus.CounterOpts{
		Name: "kafkabp_consumer_rebalance_failures_total",
		Help: "The number of times consumer rebalance failed",
	})
)

var (
	timerLabels = []string{
		topicLabel,
	}

	consumerTimer = promauto.With(prometheusbpint.GlobalRegistry).NewHistogramVec(prometheusbp.HistogramOpts{
		Name: "kafkabp_consumer_duration_seconds",
		Help: "The time took for a non-group consumer to consume a single kafka message",
	}.ToPrometheus(), timerLabels)

	groupConsumerTimer = promauto.With(prometheusbpint.GlobalRegistry).NewHistogramVec(prometheusbp.HistogramOpts{
		Name: "kafkabp_group_consumer_duration_seconds",
		Help: "The time took for a group consumer to consume a single kafka message",
	}.ToPrometheus(), timerLabels)
)

var (
	awsRackFailure = promauto.With(prometheusbpint.GlobalRegistry).NewCounter(prometheus.CounterOpts{
		Name: "kafkabp_aws_rack_id_failures_total",
		Help: "Total failures of getting rack id from AWS endpoint",
	})

	httpRackFailure = promauto.With(prometheusbpint.GlobalRegistry).NewCounter(prometheus.CounterOpts{
		Name: "kafkabp_http_rack_id_failures_total",
		Help: "Total failures of getting rack id from http endpoint",
	})
)

var (
	kafkaVersionGauge = promauto.With(prometheusbpint.GlobalRegistry).NewGaugeVec(prometheus.GaugeOpts{
		Name: "kafkabp_consumer_configured_version",
		Help: "The kafka version in kafkabp consumer config",
	}, []string{"kafka_version"})
)
