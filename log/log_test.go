package log

import (
	"testing"
)

func TestZapLogger(t *testing.T) {
	InitLogger(DebugLevel)
	logger.Debug("printing flag values")
}
