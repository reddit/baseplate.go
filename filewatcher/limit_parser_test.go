package filewatcher

import (
	"io"
	"strings"
	"testing"
)

func TestLimitParser(t *testing.T) {
	const (
		limit    = 5
		origin   = "Hello, world!"
		expected = "Hello"
	)

	parser := func(data io.Reader) (interface{}, error) {
		buf, err := io.ReadAll(data)
		if string(buf) != expected {
			t.Errorf("Data expected %q, got %q", expected, buf)
		}
		if err == nil {
			t.Error("Expected error, got nothing")
		}
		return nil, err
	}
	limitParser(parser, limit)(strings.NewReader(origin))
}
