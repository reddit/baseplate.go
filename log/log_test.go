package log

import (
	"testing"
)

func TestZapLogger(t *testing.T) {
	InitLogger(DebugLevel)
	log(globalLogger)
}
