package iobp

import (
	"io"
)

// CountingSink is an io.Writer implementation that discards all incoming data
// but tracks how many bytes were "written".
//
// This can be used as a sink for encoders or as an additional output via a
// MultiWriter to track data sizes without buffering all of the data in memory.
//
// A Write to a CountingSink cannot fail.
// A CountingSink is not safe for concurrent use.
type CountingSink int64

var _ io.Writer = (*CountingSink)(nil)

func (cs *CountingSink) Write(buf []byte) (int, error) {
	*cs += CountingSink(len(buf))
	return len(buf), nil
}

// Size returns the current counted size.
func (cs CountingSink) Size() int64 {
	return int64(cs)
}
