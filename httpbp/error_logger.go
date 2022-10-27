package httpbp

import (
	"log"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/reddit/baseplate.go/internal/prometheusbpint"
)

var httpServerLoggingCounter = promauto.With(prometheusbpint.GlobalRegistry).NewCounterVec(prometheus.CounterOpts{
	Name: "httpbp_server_upstream_issue_logs_total",
	Help: "Number of logs emitted by stdlib http server regarding an upstream issue",
}, []string{"upstream_issue"})

// This is a special zapcore used by stdlib http server to handle error logging.
type wrappedCore struct {
	zapcore.Core

	counter25192  prometheus.Counter
	suppress25192 bool
}

func (w wrappedCore) Check(entry zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	// Example message:
	//     URL query contains semicolon, which is no longer a supported separator; parts of the query may be stripped when parsed; see golang.org/issue/25192
	if strings.Contains(entry.Message, "golang.org/issue/25192") {
		w.counter25192.Inc()
		if w.suppress25192 {
			// drop the log
			return ce
		}
	}

	return w.Core.Check(entry, ce)
}

func httpServerLogger(base *zap.Logger, suppress25192 bool) (*log.Logger, error) {
	return zap.NewStdLogAt(base.WithOptions(
		zap.Fields(zap.String("from", "http-server")),
		zap.WrapCore(func(c zapcore.Core) zapcore.Core {
			return wrappedCore{
				Core: c,

				suppress25192: suppress25192,
				counter25192: httpServerLoggingCounter.With(prometheus.Labels{
					"upstream_issue": "25192",
				}),
			}
		}),
	), zapcore.WarnLevel)
}
