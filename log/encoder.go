package log

import (
	"time"

	"go.uber.org/zap/zapcore"
)

// RFC3339Nano is a time format for TimeEncoder
const RFC3339Nano = "ts=2006-01-02T15:04:05.000000Z"

// FullCallerEncoder serializes a caller in /full/path/to/package/file:line format.
func FullCallerEncoder(caller zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
	// TODO: consider using a byte-oriented API to save an allocation.
	enc.AppendString("caller=" + caller.String())
}

// ShortCallerEncoder serializes a caller in package/file:line format, trimming
// all but the final directory from the full path.
func ShortCallerEncoder(caller zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
	// TODO: consider using a byte-oriented API to save an allocation.
	enc.AppendString("caller=" + caller.TrimmedPath())
}

// TimeEncoder is customized to add ts in the front
func TimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.UTC().Format(RFC3339Nano))
}

// CapitalLevelEncoder adds logger level in uppercase
func CapitalLevelEncoder(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString("level=" + l.CapitalString())
}

// JSONTimeEncoder encodes time in RFC3339Nano without extra information.
//
// It's suitable to be used in the full JSON format.
func JSONTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.UTC().Format(time.RFC3339Nano))
}
