// +build !linux

package mqsend

func openMessageQueue(cfg MessageQueueConfig) (MessageQueue, error) {
	return OpenMockMessageQueue(cfg), nil
}
