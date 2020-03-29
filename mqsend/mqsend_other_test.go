// +build !linux

package mqsend_test

import (
	"testing"
)

func deleteMessageQueue(t *testing.T, _ string) error {
	t.Logf("WARNING: deleteMessageQueue is nop on non-linux systems")
	return nil
}
