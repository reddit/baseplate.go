package log

import (
	"strconv"

	"go.uber.org/zap/zapcore"

	"github.com/reddit/baseplate.go/internal/thriftint"
)

type wrappedCore struct {
	zapcore.Core
}

func (w wrappedCore) With(fields []zapcore.Field) zapcore.Core {
	return wrappedCore{Core: w.Core.With(wrapFields(fields))}
}

func (w wrappedCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
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
