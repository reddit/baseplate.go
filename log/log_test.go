package log

import (
	"testing"
)

func TestZapLogger(t *testing.T) {
	InitLogger(DebugLevel)
	log(globalLogger)

	Version = "test-version"
	InitLoggerJSON(DebugLevel)
	log(globalLogger)
}
