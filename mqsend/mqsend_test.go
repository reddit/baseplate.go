package mqsend_test

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/reddit/baseplate.go/mqsend"
	"github.com/reddit/baseplate.go/randbp"
)

func TestLinuxMessageQueue(t *testing.T) {
	if runtime.GOOS != `linux` || !strings.HasSuffix(runtime.GOARCH, "64") {
		t.Logf(
			"This test can only be run on 64-bit Linux, skipping on %s/%s",
			runtime.GOOS,
			runtime.GOARCH,
		)
		return
	}

	const msg = "hello, world!"
	const max = len(msg)
	const timeout = time.Millisecond

	name := fmt.Sprintf("test-mq-%d", randbp.R.Uint64())

	mq, err := mqsend.OpenMessageQueue(mqsend.MessageQueueConfig{
		Name:           name,
		MaxMessageSize: int64(max),
		MaxQueueSize:   4,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer mq.Close()

	// Delete the mq created in this test.
	defer func() {
		if err := deleteMessageQueue(t, name); err != nil {
			t.Errorf("Failed to delete message queue %q: %v", name, err)
		}
	}()

	sharedTest(t, mq, msg, max, timeout)
}
