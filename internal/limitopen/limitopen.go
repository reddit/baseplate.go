package limitopen

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	//lint:ignore SA1019 This library is internal only, not actually deprecated
	"github.com/reddit/baseplate.go/internalv2compat"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/metricsbp"
)

const (
	promNamespace = "limitopen"

	pathLabel = "path"
)

var (
	sizeLabels = []string{
		pathLabel,
	}

	sizeGauge = promauto.With(internalv2compat.GlobalRegistry).NewGaugeVec(prometheus.GaugeOpts{
		Namespace: promNamespace,
		Name:      "file_size_bytes",
		Help:      "The size of the file opened by limitopen.Open",
	}, sizeLabels)

	softLimitCounter = promauto.With(internalv2compat.GlobalRegistry).NewCounterVec(prometheus.CounterOpts{
		Namespace: promNamespace,
		Name:      "softlimit_violation_total",
		Help:      "The total number of violations of softlimit",
	}, sizeLabels)
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
// (with "limitopen.size" as the metrics path and path label for statsd,
// "limitopen_file_size_bytes" for prometheus).
// When softLimit > 0 and the size of the path as reported by the os is larger,
// it will also use log.DefaultWrapper to report it and increase prometheus
// counter of limitopen_softlimit_violation_total.
// When hardLimit > 0 and the size of the path as reported by the os is larger,
// it will close the file and return an error directly.
func OpenWithLimit(path string, softLimit, hardLimit int64) (io.ReadCloser, error) {
	r, size, err := Open(path)
	if err != nil {
		return nil, err
	}

	pathValue := filepath.Base(path)
	metricsbp.M.RuntimeGauge("limitopen.size").With(
		"path", pathValue,
	).Set(float64(size))
	labels := prometheus.Labels{
		pathLabel: pathValue,
	}
	sizeGauge.With(labels).Set(float64(size))

	if softLimit > 0 && size > softLimit {
		msg := fmt.Sprintf(
			"limitopen.OpenWithLimit: file size > soft limit, path=%q size=%d limit=%d",
			path,
			size,
			softLimit,
		)
		log.DefaultWrapper.Log(context.Background(), msg)
		softLimitCounter.With(labels).Inc()
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
