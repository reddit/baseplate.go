// +build linux

package mqsend

import (
	"context"
	"unsafe"

	"golang.org/x/sys/unix"
)

type messageQueue uintptr

// C version:
//
// struct mq_attr {
//     long mq_flags;       /* Flags (ignored for mq_open()) */
//     long mq_maxmsg;      /* Max. # of messages on queue */
//     long mq_msgsize;     /* Max. message size (bytes) */
//     long mq_curmsgs;     /* # of messages currently in queue
//                             (ignored for mq_open()) */
// };
//
// Note that this only works on 64-bit systems.
type mqAttr struct {
	_              int64
	MaxQueueSize   int64
	MaxMessageSize int64
	_              int64
}

// OpenMessageQueue opens a named message queue.
func OpenMessageQueue(cfg MessageQueueConfig) (MessageQueue, error) {
	name, err := unix.BytePtrFromString(cfg.Name)
	if err != nil {
		return nil, err
	}

	flags := unix.O_WRONLY | unix.O_CREAT

	// From MQ_OPEN(3) manpage:
	// mqd_t mq_open(const char *name, int oflag, mode_t mode, struct mq_attr *attr);
	mqd, _, errno := unix.Syscall6(
		unix.SYS_MQ_OPEN,
		uintptr(unsafe.Pointer(name)), // name
		uintptr(flags),                // oflag
		uintptr(MessageQueueOpenMode), // mode
		uintptr(unsafe.Pointer(&mqAttr{
			MaxQueueSize:   cfg.MaxQueueSize,
			MaxMessageSize: cfg.MaxMessageSize,
		})), // attr
		0, // unused
		0, // unused
	)
	if errno != 0 {
		return nil, errno
	}
	return messageQueue(mqd), nil
}

func (mqd messageQueue) Close() error {
	return unix.Close(int(mqd))
}

func (mqd messageQueue) Send(ctx context.Context, data []byte) error {
	var absTimeout uintptr
	if deadline, ok := ctx.Deadline(); ok {
		t, err := unix.TimeToTimespec(deadline)
		if err != nil {
			return err
		}
		absTimeout = uintptr(unsafe.Pointer(&t))
	}

	// From MQ_SEND(3) manpage:
	// int mq_timedsend(mqd_t mqdes, const char *msg_ptr, size_t msg_len, unsigned int msg_prio, const struct timespec *abs_timeout);
	_, _, errno := unix.Syscall6(
		unix.SYS_MQ_TIMEDSEND,
		uintptr(mqd),                      // mqdes
		uintptr(unsafe.Pointer(&data[0])), // msg_ptr
		uintptr(len(data)),                // msg_len
		0,                                 // msg_prio
		absTimeout,                        // abs_timeout
		0,                                 // unused
	)
	if errno == 0 {
		return nil
	}
	return errno
}
