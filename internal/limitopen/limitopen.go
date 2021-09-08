package limitopen

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/metricsbp"
)

// Open opens a path for read.
//
// It's similar to os.Open, but with the following differences:
//
// 1. It returns io.ReadCloser other than *os.File,
//
// 2. It returns the size reported by the system to the user.
//
// 3. The io.ReadCloser returned would be guaranteed to be not able to read
// beyond the size returned (for example, if you use this function to open
// /dev/zero, the system would return 0 as the size, and as a result when
// reading from r you would get EOF immediately, instead of being able to
// read from it indefinitely)
//
// It would never return both non-nil r and err.
// When err is nil it's the caller's responsibility to close r returned.
func Open(path string) (r io.ReadCloser, size int64, err error) {
	var f *os.File
	f, err = os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("limitopen.Open: failed to open file %q: %w", path, err)
	}

	defer func() {
		if err != nil {
			f.Close()
		}
	}()

	var stats fs.FileInfo
	stats, err = f.Stat()
	if err != nil {
		return nil, 0, fmt.Errorf("limitopen.Open: failed to get the size of %q: %w", path, err)
	}

	size = stats.Size()
	return readCloser{
		Reader: io.LimitReader(f, size),
		Closer: f,
	}, size, err
}

type readCloser struct {
	io.Reader
	io.Closer
}

// OpenWithLimit calls Open with limit checks.
//
// It always reports the size of the path as a runtime gauge
// (with "limitopen.size" as the metrics path and path label).
// When softLimit > 0 and the size of the path as reported by the os is larger,
// it will also use log.ErrorWithSentry to report it.
// When hardLimit > 0 and the size of the path as reported by the os is larger,
// it will close the file and return an error directly.
func OpenWithLimit(path string, softLimit, hardLimit int64) (io.ReadCloser, error) {
	r, size, err := Open(path)
	if err != nil {
		return nil, err
	}

	metricsbp.M.RuntimeGauge("limitopen.size").With(
		"path", filepath.Base(path),
	).Set(float64(size))

	if softLimit > 0 && size > softLimit {
		const msg = "limitopen.OpenWithLimit: file size > soft limit"
		log.ErrorWithSentry(
			context.Background(),
			msg,
			errors.New(msg),
			"path", path,
			"size", size,
			"limit", softLimit,
		)
	}

	if hardLimit > 0 && size > hardLimit {
		r.Close()
		return nil, fmt.Errorf(
			"limitopen.OpenWithLimit: file size %d > hard limit %d for path %q",
			size,
			hardLimit,
			path,
		)
	}

	return r, nil
}
