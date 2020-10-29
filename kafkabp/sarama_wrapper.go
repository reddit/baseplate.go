package kafkabp

import (
	"errors"
)

// Allowed Offset values
const (
	OffsetOldest = "oldest"
	OffsetNewest = "newest"
)

var (
	// ErrBrokersEmpty is thrown when the slice of brokers is empty.
	ErrBrokersEmpty = errors.New("kafkabp: Brokers are empty")

	// ErrTopicEmpty is thrown when the topic is empty.
	ErrTopicEmpty = errors.New("kafkabp: Topic is empty")

	// ErrClientIDEmpty is thrown when the client ID is empty.
	ErrClientIDEmpty = errors.New("kafkabp: ClientID is empty")

	// ErrOffsetInvalid is thrown when an invalid offset is specified.
	ErrOffsetInvalid = errors.New("kafkabp: Offset is invalid")
)
