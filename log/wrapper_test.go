package log_test

import (
	"context"
	"testing"

	"github.com/reddit/baseplate.go/log"
)

func TestLogWrapperNilSafe(t *testing.T) {
	// Just make sure log.Wrapper.Log is nil-safe, no real tests
	var logger log.Wrapper
	logger.Log(context.Background(), "Hello, world!")
	logger.ToThriftLogger()("Hello, world!")
	log.WrapToThriftLogger(nil)("Hello, world!")
}
