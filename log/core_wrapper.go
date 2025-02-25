package log

import (
	"strconv"
	"time"

	"go.uber.org/zap/zapcore"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/reddit/baseplate.go/internal/prometheusbpint"
	"github.com/reddit/baseplate.go/internal/thriftint"
)

var (
	logWriteDurationSeconds = promauto.With(prometheusbpint.GlobalRegistry).NewHistogram(
		prometheus.HistogramOpts{
			Name:    "baseplate_log_write_duration_seconds",
			Help:    "Latency of log calls",
			Buckets: []float64{
				0.000_005,
				0.000_010,
				0.000_050,
				0.000_100,
				0.000_500,
				0.001,
				0.005,
				0.01,
				0.05,
				0.1,
				0.5,
				1.0,
			},
		},
	)
)

type wrappedCore struct {
	zapcore.Core
}

func (w wrappedCore) With(fields []zapcore.Field) zapcore.Core {
	return wrappedCore{Core: w.Core.With(wrapFields(fields))}
}

func (w wrappedCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	defer func(start time.Time) {
		logWriteDurationSeconds.Observe(time.Since(start).Seconds())
	}(time.Now())
	return w.Core.Write(entry, wrapFields(fields))
}

func (w wrappedCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if w.Enabled(ent.Level) {
		return ce.AddCore(ent, w)
	}
	return ce
}

func wrapFields(fields []zapcore.Field) []zapcore.Field {
	for i, f := range fields {
		switch f.Type {
		// To make sure larger int64/uint64 logged will not be treated as float64
		// and lose precision.
		case zapcore.Int64Type:
			f.Type = zapcore.StringType
			f.String = strconv.FormatInt(f.Integer, 10)
		case zapcore.Uint64Type:
			f.Type = zapcore.StringType
			f.String = strconv.FormatUint(uint64(f.Integer), 10)

		// To make *baseplate.Errors more human readable in the logs.
		case zapcore.ErrorType:
			f.Interface = thriftint.WrapBaseplateError(f.Interface.(error))
		}
		fields[i] = f
	}
	return fields
}
