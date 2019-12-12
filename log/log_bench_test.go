package log

import (
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	ifacezap "github.com/go-kit/kit/log/zap"
)

func BenchmarkStructuredLogger(b *testing.B) {
	// run the Fib function b.N times
	InitLogger(NopLevel)
	for n := 0; n < b.N; n++ {
		logger.Debug("this is a test")
	}
}

func BenchmarkInterfacedLogger(b *testing.B) {
	// run the Fib function b.N times
	logger := ifacezap.NewZapSugarLogger(zap.NewNop(), zapcore.InfoLevel)
	for n := 0; n < b.N; n++ {
		logger.Log("this is a test")
	}
}
