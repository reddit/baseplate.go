package limitreader

import (
	"bufio"
	"errors"
	"io"
)

// ErrReadLimitExceeded is the error returned when there's more data on the
// underlying reader and a read operation is trying to read beyond that.
var ErrReadLimitExceeded = errors.New("limitreader: read limit exceeded")

// New wraps reader with a limit.
//
// It's similar to io.LimitReader,
// but instead of always returning io.EOF when read beyond the limit,
// it would return a different error (which would cause the read operation to
// fail) when there's more data on the underlying reader.
func New(r io.Reader, limit int64) io.Reader {
	return &reader{
		reader:    bufio.NewReaderSize(r, 16),
		remaining: limit,
	}
}

type reader struct {
	reader    *bufio.Reader
	remaining int64
}

func (r *reader) Read(p []byte) (int, error) {
	// EOF handling
	if r.remaining <= 0 {
		_, err := r.reader.Peek(1)
		if err != nil {
			// NOTE: This also includes io.EOF error, which is the expected behavior.
			return 0, err
		}
		return 0, ErrReadLimitExceeded
	}

	if len(p) > int(r.remaining) {
		p = p[:r.remaining]
	}
	read, err := r.reader.Read(p)
	r.remaining -= int64(read)
	return read, err
}
