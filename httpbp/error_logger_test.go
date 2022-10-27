package httpbp

import (
	"io"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/reddit/baseplate.go/prometheusbp/promtest"
)

func initBaseLogger(w io.Writer) *zap.Logger {
	// Mostly copied from zap.NewExample, to make the log deterministic.
	encoderCfg := zapcore.EncoderConfig{
		MessageKey:     "msg",
		LevelKey:       "level",
		NameKey:        "logger",
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
	}
	core := zapcore.NewCore(zapcore.NewJSONEncoder(encoderCfg), zapcore.AddSync(w), zap.DebugLevel)
	return zap.New(core)
}

func TestHTTPServerLogger(t *testing.T) {
	t.Run("suppressed", func(t *testing.T) {
		defer promtest.NewPrometheusMetricTest(t, "25192", httpServerLoggingCounter, prometheus.Labels{
			"upstream_issue": "25192",
		}).CheckDelta(2)

		var sb strings.Builder
		logger, err := httpServerLogger(initBaseLogger(&sb), true)
		if err != nil {
			t.Fatalf("httpServerLogger failed: %v", err)
		}
		logger.Printf("Hello, golang.org/issue/25192!")
		logger.Printf("Hello, golang.org/issue/25192!")
		if str := sb.String(); strings.TrimSpace(str) != "" {
			t.Errorf("Expected logs being suppressed, got %q", str)
		}

		sb.Reset()
		logger.Printf("Hello, world!")
		const want = `{"level":"warn","msg":"Hello, world!","from":"http-server"}`
		if got := sb.String(); strings.TrimSpace(got) != want {
			t.Errorf("Got %q, want %q", got, want)
		}
	})

	t.Run("not-suppressed", func(t *testing.T) {
		defer promtest.NewPrometheusMetricTest(t, "25192", httpServerLoggingCounter, prometheus.Labels{
			"upstream_issue": "25192",
		}).CheckDelta(1)

		var sb strings.Builder
		logger, err := httpServerLogger(initBaseLogger(&sb), false)
		if err != nil {
			t.Fatalf("httpServerLogger failed: %v", err)
		}
		logger.Printf("Hello, golang.org/issue/25192!")
		if got, want := sb.String(), `{"level":"warn","msg":"Hello, golang.org/issue/25192!","from":"http-server"}`; strings.TrimSpace(got) != want {
			t.Errorf("Got %q, want %q", got, want)
		}

		sb.Reset()
		logger.Printf("Hello, world!")
		if got, want := sb.String(), `{"level":"warn","msg":"Hello, world!","from":"http-server"}`; strings.TrimSpace(got) != want {
			t.Errorf("Got %q, want %q", got, want)
		}
	})
}
