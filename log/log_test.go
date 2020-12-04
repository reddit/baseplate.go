package log

import (
	"testing"
)

func TestZapLogger(t *testing.T) {
	InitLoggerJSON(DebugLevel)
	globalLogger.Debugw("This is a log", "int64", 123)
}
