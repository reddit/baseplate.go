package log

import (
	"testing"
)

func TestZapLogger(t *testing.T) {
	InitLogger(DebugLevel)
	globalLogger.Debug("printing flag values")
}
