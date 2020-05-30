package log_test

import (
	"testing"

	"github.com/reddit/baseplate.go/log"
)

func TestLogWrapperNilSafe(t *testing.T) {
	// Just make sure log.Wrapper.Log is nil-safe, no real tests
	var logger log.Wrapper
	logger.Log("Hello, world!")
}
