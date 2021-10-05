package log

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"go.uber.org/zap"
)

func TestZapWrapper(t *testing.T) {
	const (
		yamlLine        = "zap:debug:key with space=value with space"
		expectedLogLine = `{"level":"debug","msg":"This is a log","key with space":"value with space"}`
	)

	var w Wrapper
	if err := w.UnmarshalText([]byte(yamlLine)); err != nil {
		t.Fatalf("Failed to parse yaml line %q: %v", yamlLine, err)
	}

	buf := new(bytes.Buffer)
	core := initCore(buf)
	ctx := context.WithValue(context.Background(), contextKey, zap.New(core).Sugar())
	w.Log(ctx, "This is a log")
	actual := strings.TrimSpace(buf.String())
	if actual != expectedLogLine {
		t.Errorf("Expected log line %s, got %s", expectedLogLine, actual)
	}
}
