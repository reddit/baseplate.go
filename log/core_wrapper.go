package log

import (
	"strconv"

	"go.uber.org/zap/zapcore"
)

type wrappedCore struct {
	zapcore.Core
}

func (w wrappedCore) With(fields []zapcore.Field) zapcore.Core {
	return wrappedCore{Core: w.Core.With(fieldsInt64ToString(fields))}
}

func (w wrappedCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	return w.Core.Write(entry, fieldsInt64ToString(fields))
}

func (w wrappedCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if w.Enabled(ent.Level) {
		return ce.AddCore(ent, w)
	}
	return ce
}

func fieldsInt64ToString(fields []zapcore.Field) []zapcore.Field {
	for i, f := range fields {
		switch f.Type {
		case zapcore.Int64Type:
			f.Type = zapcore.StringType
			f.String = strconv.FormatInt(f.Integer, 10)
		case zapcore.Uint64Type:
			f.Type = zapcore.StringType
			f.String = strconv.FormatUint(uint64(f.Integer), 10)
		}
		fields[i] = f
	}
	return fields
}
