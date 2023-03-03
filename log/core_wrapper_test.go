package log

import (
	"bytes"
	"strings"
	"testing"

	"github.com/apache/thrift/lib/go/thrift"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
)

var bpErr = &baseplate.Error{
	Message: thrift.StringPtr("foo"),
	Code:    thrift.Int32Ptr(1),
}

func log(l *zap.SugaredLogger) {
	l.Debugw(
		"This is a log",
		"int", int(123),
		"int64", int64(1234),
		"uint64", uint64(1234),
		"bpErr", bpErr,
	)
}

func initCore(buf *bytes.Buffer) zapcore.Core {
	// Mostly copied from zap.NewExample, to make the log deterministic.
	encoderCfg := zapcore.EncoderConfig{
		MessageKey:     "msg",
		LevelKey:       "level",
		NameKey:        "logger",
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
	}
	return zapcore.NewCore(zapcore.NewJSONEncoder(encoderCfg), zapcore.AddSync(buf), zap.DebugLevel)
}

func TestWrappedCore(t *testing.T) {
	const (
		// Example:
		// {"level":"debug","msg":"This is a log","int":123,"int64":1234,"uint64":1234,"bpErr":"Error({Code:0xc0000268bc Message:0xc000072390 Details:map[] Retryable:<nil>})"}
		expectedOriginPrefix = `{"level":"debug","msg":"This is a log","int":123,"int64":1234,"uint64":1234,"bpErr":"Error({Code:0x`

		expectedWrapped = `{"level":"debug","msg":"This is a log","int":"123","int64":"1234","uint64":"1234","bpErr":"baseplate.Error: \"foo\" (code=1)"}`
	)
	t.Run("origin", func(t *testing.T) {
		buf := new(bytes.Buffer)
		core := initCore(buf)
		logger := zap.New(core).Sugar()
		log(logger)
		actual := strings.TrimSpace(buf.String())
		if !strings.HasPrefix(actual, expectedOriginPrefix) {
			t.Errorf("Expected log line to start with %#q, got %#q", expectedOriginPrefix, actual)
		}
	})
	t.Run("wrapped", func(t *testing.T) {
		buf := new(bytes.Buffer)
		core := wrappedCore{initCore(buf)}
		logger := zap.New(core).Sugar()
		log(logger)
		actual := strings.TrimSpace(buf.String())
		if actual != expectedWrapped {
			t.Errorf("Expected log line %#q, got %#q", expectedWrapped, actual)
		}
	})
	t.Run("wrapped-with", func(t *testing.T) {
		buf := new(bytes.Buffer)
		core := wrappedCore{initCore(buf)}
		logger := zap.New(core).Sugar()
		logger = logger.With("int", int(123))
		logger.Debugw(
			"This is a log",
			"int64", int64(1234),
			"uint64", uint64(1234),
			"bpErr", bpErr,
		)
		actual := strings.TrimSpace(buf.String())
		if actual != expectedWrapped {
			t.Errorf("Expected log line %#q, got %#q", expectedWrapped, actual)
		}
	})
}

func BenchmarkWrappedCore(b *testing.B) {
	b.Run("origin", func(b *testing.B) {
		buf := new(bytes.Buffer)
		core := initCore(buf)
		logger := zap.New(core).Sugar()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			buf.Reset()
			log(logger)
		}
	})

	b.Run("wrapped", func(b *testing.B) {
		buf := new(bytes.Buffer)
		core := wrappedCore{initCore(buf)}
		logger := zap.New(core).Sugar()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			buf.Reset()
			log(logger)
		}
	})
}
