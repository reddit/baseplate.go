// +build !linux

package mqsend

// OpenMessageQueue opens a named message queue.
//
// On non-linux systems this just returns a mocked version,
// see OpenMockMessageQueue.
func OpenMessageQueue(cfg MessageQueueConfig) (MessageQueue, error) {
	return OpenMockMessageQueue(cfg), nil
}
