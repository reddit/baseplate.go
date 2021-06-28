package mqsend

import (
	"fmt"
	"strings"
)

// TimedOutError is the error returned by MessageQueue.Send when the operation
// timed out because of the queue was full.
//
// On linux systems it usually wraps one of syscall.ETIMEDOUT, syscall.EAGAIN,
// context.Canceled, or context.DeadlineExceeded.
// On other systems (or with MockMessageQueue) it usually wraps either
// context.Canceled or context.DeadlineExceeded.
type TimedOutError struct {
	Cause error
}

func (e TimedOutError) Error() string {
	return fmt.Sprintf("mqsend: send timed out: %v", e.Cause)
}

// Unwrap returns the underlying error.
func (e TimedOutError) Unwrap() error {
	return e.Cause
}

// MessageTooLargeError is the error returned by MessageQueue.Send when the
// message is larger than the configured max size.
//
// On linux systems it wraps syscall.EMSGSIZE, on other systems (or with
// MockMessageQueue) it doesn't wrap any other errors.
type MessageTooLargeError struct {
	MessageSize int

	// Note that MaxSize will always be 0 on linux systems,
	// as we don't store it after send to mq_open syscall.
	MaxSize int

	// Note that on non-linux systems Cause will be nil.
	Cause error
}

func (e MessageTooLargeError) Error() string {
	var sb strings.Builder
	sb.WriteString("mqsend: message too large")
	if e.MaxSize != 0 {
		sb.WriteString(fmt.Sprintf(" (%d > %d)", e.MessageSize, e.MaxSize))
	} else {
		sb.WriteString(fmt.Sprintf(" (%d)", e.MessageSize))
	}
	if e.Cause != nil {
		sb.WriteString(": ")
		sb.WriteString(e.Cause.Error())
	}
	return sb.String()
}

// Unwrap returns the underlying error, if any.
func (e MessageTooLargeError) Unwrap() error {
	return e.Cause
}
