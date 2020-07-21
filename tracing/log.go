package tracing

import (
	"context"
	"testing"

	"github.com/reddit/baseplate.go/log"
)

// TestWrapper is a log.Wrapper implementation can be used in tests.
//
// It's similar to but different from log.TestWrapper.
// In InitGlobalTracer call logger could be called once for unable to find ip,
// and we don't want to fail the tests because of that.
// So in this implementation,
// initially this logger just print the message but don't fail the test,
// and only start failing the test after startFailing is called.
func TestWrapper(tb testing.TB) (logger log.Wrapper, startFailing func()) {
	logf := tb.Logf
	logger = func(_ context.Context, msg string) {
		logf("logger called with msg: %q", msg)
	}
	startFailing = func() {
		logf = tb.Errorf
	}
	return
}
