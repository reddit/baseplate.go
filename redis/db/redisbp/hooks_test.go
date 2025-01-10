package redisbp

import (
	"context"
	"github.com/reddit/baseplate.go/internal/prometheusbpint"
	"github.com/stretchr/testify/assert"
	"testing"

	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"

	"github.com/reddit/baseplate.go/prometheusbp/promtest"
	"github.com/reddit/baseplate.go/redis/internal/redisprom"
	"github.com/reddit/baseplate.go/thriftbp"
	"github.com/reddit/baseplate.go/tracing"
)

func TestSpanHook(t *testing.T) {
	ctx, _ := thriftbp.StartSpanFromThriftContext(context.Background(), "foo")
	hooks := SpanHook{
		ClientName: "redis",
		Type:       "type",
		Deployment: "Deployment",
		Cluster:    "cluster",
	}
	statusCmd := redis.NewStatusCmd(ctx, "ping")
	t.Run(
		"ProcessHook",
		func(t *testing.T) {
			hook := SpanHook{
				promActive: &prometheusbpint.HighWatermarkGauge{
					HighWatermarkValue:   &prometheusbpint.HighWatermarkValue{},
					CurrGauge:            redisprom.ActiveConnectionsDesc,
					CurrGaugeLabelValues: []string{"test"},
					MaxGauge:             redisprom.PeakActiveConnectionsDesc,
					MaxGaugeLabelValues:  []string{"test"},
				},
			}
			ph := func(ctx context.Context, cmd redis.Cmder) error { return nil }
			err := hook.ProcessHook(ph)(context.Background(), statusCmd)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			assert.Equal(t, int64(1), hook.promActive.Max())
		},
	)
	t.Run(
		"Before/AfterProcess",
		func(t *testing.T) {
			defer promtest.NewPrometheusMetricTest(t, "latency timer", latencyTimer, prometheus.Labels{
				nameLabel:    "redis",
				commandLabel: "ping",
				successLabel: "true",
			}).CheckSampleCountDelta(1)
			labels := prometheus.Labels{
				redisprom.ClientNameLabel: "redis",
				redisprom.TypeLabel:       "type",
				redisprom.CommandLabel:    "ping",
				redisprom.DeploymentLabel: "Deployment",
				redisprom.SuccessLabel:    "true",
				redisprom.DatabaseLabel:   "",
				redisprom.ClusterLabel:    "cluster",
			}
			defer promtest.NewPrometheusMetricTest(t, "spec latency timer", redisprom.LatencySeconds, labels).CheckSampleCountDelta(1)
			defer promtest.NewPrometheusMetricTest(t, "spec requests total", redisprom.RequestsTotal, labels).CheckDelta(1)

			ctx := hooks.startChildSpan(ctx, statusCmd.Name())
			activeSpan := opentracing.SpanFromContext(ctx)
			if activeSpan == nil {
				t.Fatalf("'activeSpan' is 'nil'")
			}
			if name := tracing.AsSpan(activeSpan).Name(); name != "redis.ping" {
				t.Fatalf("Incorrect span name %q", name)
			}

			hooks.endChildSpan(ctx, nil)
		},
	)
}
