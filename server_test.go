package baseplate

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/reddit/baseplate.go/log"
)

func floatsEqual(a, b float64) bool {
	const epsilon = 1e-9
	return math.Abs(a-b) <= epsilon
}

func TestParseServerconfig(t *testing.T) {
	const yaml = `
addr: ":6060"
# duration
timeout: 10ms

log:
  level: "info"

metrics:
  histogramsamplerate: 0.1
  # countersamplerate is missing so it's supposed to be nil
`

	cfg, err := parseConfigYaml(strings.NewReader(yaml))
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Parsed cfg: %#v", cfg)

	const expectedAddr = ":6060"
	if cfg.Addr != expectedAddr {
		t.Errorf(
			"Expected Addr %q, got %q",
			expectedAddr,
			cfg.Addr,
		)
	}

	const expectedTimeout = time.Millisecond * 10
	if cfg.Timeout != expectedTimeout {
		t.Errorf(
			"Expected Timeout %v, got %v",
			expectedTimeout,
			cfg.Timeout,
		)
	}

	const expectedLevel = log.InfoLevel
	if cfg.Log.Level != expectedLevel {
		t.Errorf(
			"Expected Log.Level %v, got %v",
			expectedLevel,
			cfg.Log.Level,
		)
	}

	const expectedHistogramSampleRate = 0.1
	if cfg.Metrics.HistogramSampleRate == nil {
		t.Error("Expected non-nil Metrics.HistogramSampleRate, got nil")
	} else if !floatsEqual(expectedHistogramSampleRate, *cfg.Metrics.HistogramSampleRate) {
		t.Errorf(
			"Expected Metrics.HistogramSampleRate %v, got %v",
			expectedHistogramSampleRate,
			*cfg.Metrics.HistogramSampleRate,
		)
	}

	if cfg.Metrics.CounterSampleRate != nil {
		t.Errorf(
			"Expected nil Metrics.CounterSampleRate, got %v",
			*cfg.Metrics.CounterSampleRate,
		)
	}
}
