package log

import (
	"os"
	"testing"

	"github.com/go-kit/kit/log"
)

func TestKitLogger(t *testing.T) {
	logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout))
	logger = log.With(logger, "ts", log.DefaultTimestampUTC, "caller", log.DefaultCaller)

	logger.Log("msg", "printing flag values")
}

func TestZapLogger(t *testing.T) {
	InitLogger(DebugLevel)
	logger.Debug("printing flag values")
}
