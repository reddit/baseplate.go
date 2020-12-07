package log

import (
	"bytes"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func log(l *zap.SugaredLogger) {
	l.Debugw(
		"This is a log",
		"int", int(123),
		"int64", int64(1234),
		"uint64", uint64(1234),
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
		expectedOrigin  = `{"level":"debug","msg":"This is a log","int":123,"int64":1234,"uint64":1234}`
		expectedWrapped = `{"level":"debug","msg":"This is a log","int":"123","int64":"1234","uint64":"1234"}`
	)
	t.Run("origin", func(t *testing.T) {
		buf := new(bytes.Buffer)
		core := initCore(buf)
		logger := zap.New(core).Sugar()
		log(logger)
		actual := strings.TrimSpace(buf.String())
		if actual != expectedOrigin {
			t.Errorf("Expected log line %s, got %s", expectedOrigin, actual)
		}
	})
	t.Run("wrapped", func(t *testing.T) {
		buf := new(bytes.Buffer)
		core := wrappedCore{initCore(buf)}
		logger := zap.New(core).Sugar()
		log(logger)
		actual := strings.TrimSpace(buf.String())
		if actual != expectedWrapped {
			t.Errorf("Expected log line %s, got %s", expectedWrapped, actual)
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
		)
		actual := strings.TrimSpace(buf.String())
		if actual != expectedWrapped {
			t.Errorf("Expected log line %s, got %s", expectedWrapped, actual)
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
