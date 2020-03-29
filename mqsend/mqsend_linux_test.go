// +build linux

package mqsend_test

import (
	"testing"
	"unsafe"

	"golang.org/x/sys/unix"
)

func deleteMessageQueue(t *testing.T, name string) error {
	nameBytes, err := unix.BytePtrFromString(name)
	if err != nil {
		return err
	}
	// From MQ_UNLiNK(3) manpage:
	// int mq_unlink(const char *name);
	_, _, errno := unix.Syscall(
		unix.SYS_MQ_UNLINK,
		uintptr(unsafe.Pointer(nameBytes)), // name
		0,                                  // unused
		0,                                  // unused
	)
	if errno != 0 {
		return errno
	}
	return nil
}
