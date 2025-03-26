package filewatcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/reddit/baseplate.go/internal/limitopen"
)

// Default option values
const (
	// DefaultFSEventsDelay is the default FSEventsDelay used when creating a new
	// FileWatcher.
	DefaultFSEventsDelay = 1 * time.Second

	// DefaultPollingInterval is the default PollingInterval used when creating a
	// new FileWatcher.
	DefaultPollingInterval = 30 * time.Second

	// DefaultInitialReadInterval is the default InitialReadInterval used when
	// creating a new FileWatcher.
	DefaultInitialReadInterval = time.Second / 2

	// DefaultMaxFileSize is the default MaxFileSize used when it's <= 0.
	//
	// It's 10 MiB, with hard limit multiplier of 10.
	DefaultMaxFileSize  = 10 << 20
	HardLimitMultiplier = 10
)

// FileWatcher loads and parses data from a file or directory, and watches for
// changes in order to refresh its stored data.
type FileWatcher[T any] interface {
	// Close stops the FileWatcher
	//
	// After Close is called you won't get any updates on the file content,
	// but you can still call Get to get the last content before stopping.
	//
	// It's OK to call Close multiple times.
	// Calls after the first one are essentially no-op.
	//
	// Close never return an error.
	io.Closer

	// Get returns the latest, parsed data from the FileWatcher.
	Get() T
}

// A Parser is a callback function to be called when a watched file has its
// content changed, or is read for the first time.
type Parser[T any] func(f io.Reader) (data T, err error)

// A DirParser is a callback function that will be called when the watched
// directory has its content changed or is read for the first time.
//
// Use WrapDirParser to wrap it into a Parser to be used with FileWatcher.
type DirParser[T any] func(dir fs.FS) (data T, err error)

// WrapDirParser wraps a DirParser for a directory to a Parser.
//
// When using FileWatcher to watch a directory instead of a single file,
// you MUST use WrapDirParser instead of any other Parser implementations.
func WrapDirParser[T any](dp DirParser[T]) Parser[T] {
	return func(r io.Reader) (data T, err error) {
		path := string(r.(fakeDirectoryReader))
		dir := os.DirFS(path)
		return dp(dir)
	}
}

// Result is the return type of New. Use Get function to get the actual data.
type Result[T any] struct {
	data atomic.Pointer[dataAt[T]]

	ctx    context.Context
	cancel context.CancelFunc
}

// Get returns the latest parsed data from the FileWatcher.
func (r *Result[T]) Get() T {
	return r.data.Load().data
}

// Close stops the FileWatcher.
//
// After Close is called you won't get any updates on the file content,
// but you can still call Get to get the last content before stopping.
//
// It's OK to call Close multiple times.
// Calls after the first one are essentially no-op.
//
// Close never returns an error.
func (r *Result[T]) Close() error {
	r.cancel()
	return nil
}

func getMtime(path string) (time.Time, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return stat.ModTime(), nil
}

func (r *Result[T]) watcherLoop(
	watcher *fsnotify.Watcher,
	path string,
	parser Parser[T],
	softLimit, hardLimit int64,
	pollingInterval time.Duration,
	fsEventsDelay time.Duration,
) {
	var lock sync.Mutex
	forceReload := func() {
		// make sure we don't run forceReload concurrently
		lock.Lock()
		defer lock.Unlock()

		d, mtime, files, err := openAndParse(path, parser, softLimit, hardLimit)
		if err != nil {
			slog.ErrorContext(r.ctx, "filewatcher: openAndParse returned error", "err", err)
			return
		}
		r.data.Store(&dataAt[T]{
			data:  d,
			mtime: mtime,
		})
		// remove all previously watched files
		for _, path := range watcher.WatchList() {
			watcher.Remove(path)
		}
		// then read all new files to watch
		for _, path := range files {
			// Evaluate any symlinks before adding to work around https://github.com/fsnotify/fsnotify/issues/652
			real, err := filepath.EvalSymlinks(path)
			if err != nil {
				slog.ErrorContext(r.ctx, "filewatcher: failed to evaluate symlinks, using original name", "err", err, "path", path)
				real = path
			}
			if err := watcher.Add(real); err != nil {
				slog.ErrorContext(r.ctx, "filewatcher: failed to watch file", "err", err, "path", path, "real", real)
			}
		}
	}

	reload := func() {
		mtime, err := getMtime(path)
		if err != nil {
			slog.ErrorContext(r.ctx, "filewatcher: failed to get mtime for file", "err", err, "path", path)
			return
		}
		if r.data.Load().mtime.Before(mtime) {
			forceReload()
		}
	}

	var tickerChan <-chan time.Time
	if pollingInterval > 0 {
		ticker := time.NewTicker(pollingInterval)
		defer ticker.Stop()
		tickerChan = ticker.C
	}
	var timer *time.Timer // only populated with time.AfterFunc
	for {
		select {
		case <-r.ctx.Done():
			watcher.Close()
			return

		case err := <-watcher.Errors:
			slog.ErrorContext(r.ctx, "filewatcher: watcher error", "err", err)

		case ev := <-watcher.Events:
			// When both r.ctx.Done() and watcher.Events are unblocked, there's no
			// guarantee which case would be picked, so do an additional ctx check
			// here to make sure we don't spam the log with i/o errors (which mainly
			// happen in tests)
			if r.ctx.Err() != nil {
				continue
			}

			switch ev.Op {
			default:
				// Ignore uninterested events.
			case fsnotify.Create, fsnotify.Write, fsnotify.Rename, fsnotify.Remove:
				// Use fsEventDelay to avoid calling forceReload repetively when a burst
				// of fs events happens (for example, when multiple files within the
				// directory changed).
				if timer == nil {
					timer = time.AfterFunc(fsEventsDelay, forceReload)
				} else {
					// Appropriate here without the additional check because timer was
					// created via time.AfterFunc not time.NewTimer.
					// See the discussion here:
					// https://github.com/reddit/baseplate.go/pull/654#discussion_r1610983058
					timer.Reset(fsEventsDelay)
				}
			}

		case <-tickerChan:
			// When both r.ctx.Done() and tickerChan are unblocked, there's no
			// guarantee which case would be picked, so do an additional ctx check
			// here to make sure we don't spam the log with i/o errors (which mainly
			// happen in tests)
			if r.ctx.Err() != nil {
				continue
			}

			reload()
		}
	}
}

// The actual data and mtime held in Result.data.
type dataAt[T any] struct {
	// actual parsed data
	data T

	// other metadata
	mtime time.Time
}

var (
	_ FileWatcher[any] = (*Result[any])(nil)
)

type fakeDirectoryReader string

func (fakeDirectoryReader) Read([]byte) (int, error) {
	return 0, errors.New("filewatcher: you are most likely watching a directory without using DirParser")
}

func openAndParse[T any](path string, parser Parser[T], limit, hardLimit int64) (data T, mtime time.Time, files []string, _ error) {
	var zero T
	stats, err := os.Stat(path)
	if err != nil {
		return zero, time.Time{}, nil, fmt.Errorf("filewatcher: i/o error: %w", err)
	}
	mtime = stats.ModTime()
	files = []string{
		// Note: We need to also watch the parent directory,
		// because only watching the file won't give us CREATE events,
		// which will happen with atomic renames.
		filepath.Dir(path),
		path,
	}

	var reader io.Reader
	if stats.IsDir() {
		reader = fakeDirectoryReader(path)
		if err := filepath.Walk(path, func(p string, _ fs.FileInfo, err error) error {
			if err == nil {
				files = append(files, p)
			}
			return nil
		}); err != nil {
			return zero, time.Time{}, nil, fmt.Errorf("filewatcher: i/o error: %w", err)
		}
	} else {
		// file
		f, err := limitopen.OpenWithLimit(path, limit, hardLimit)
		if err != nil {
			return zero, time.Time{}, nil, fmt.Errorf("filewatcher: i/o error: %w", err)
		}
		defer f.Close()
		reader = f
	}

	d, err := parser(reader)
	if err != nil {
		return zero, time.Time{}, nil, fmt.Errorf("filewatcher: parser error: %w", err)
	}
	return d, mtime, files, nil
}

type opts struct {
	fsEventsDelay       time.Duration
	pollingInterval     time.Duration
	initialReadInterval time.Duration

	fileSizeLimit int64
}

// Option used in New.
type Option func(*opts)

// WithOptions is a sugar to curry zero or more options.
func WithOptions(options ...Option) Option {
	return func(o *opts) {
		for _, opt := range options {
			opt(o)
		}
	}
}

// WithFSEventsDelay sets the delay between receiving the fs events and actually
// reading and parsing the changes.
//
// It's used to avoid short bursts of fs events (for example, when watching a
// directory) causing reading and parsing repetively.
//
// Defaut to DefaultFSEventsDelay.
func WithFSEventsDelay(delay time.Duration) Option {
	return func(o *opts) {
		o.fsEventsDelay = delay
	}
}

// WithPollingInterval sets the interval to check file changes proactively.
//
// Default to DefaultPollingInterval.
// To disable polling completely, set it to a negative value.
//
// Without polling, filewatcher relies solely on fs events from the parent
// directory. This works for most cases but will not work in the cases that
// the parent directory will be remount upon change
// (for example, k8s ConfigMap).
func WithPollingInterval(interval time.Duration) Option {
	return func(o *opts) {
		o.pollingInterval = interval
	}
}

// WithInitialReadInterval sets the interval to keep retrying to open the file
// when creating a new FileWatcher, when the file was not initially available.
//
// Default to DefaultInitialReadInterval.
func WithInitialReadInterval(interval time.Duration) Option {
	return func(o *opts) {
		o.initialReadInterval = interval
	}
}

// WithFileSizeLimit sets the soft file size limit, with the hard limit being
// 10x (see HardLimitMultiplier) of the set soft limit.
//
// This is completely ignored when DirParser is used.
//
// If the soft limit is violated,
// the violation will be reported via slog at error level and prometheus
// counter of limitopen_softlimit_violation_total,
// but it does not stop the normal parsing process.
//
// If the hard limit is violated,
// The loading of the file will fail immediately.
//
// Default to DefaultMaxFileSize.
func WithFileSizeLimit(limit int64) Option {
	return func(o *opts) {
		o.fileSizeLimit = limit
	}
}

func defaultOptions() Option {
	return WithOptions(
		WithFSEventsDelay(DefaultFSEventsDelay),
		WithPollingInterval(DefaultPollingInterval),
		WithInitialReadInterval(DefaultInitialReadInterval),
		WithFileSizeLimit(DefaultMaxFileSize),
	)
}

// New creates a new FileWatcher.
//
// If the path is not available at the time of calling,
// it blocks until the file becomes available, or context is cancelled,
// whichever comes first.
func New[T any](ctx context.Context, path string, parser Parser[T], options ...Option) (*Result[T], error) {
	var opt opts
	WithOptions(
		defaultOptions(),
		WithOptions(options...),
	)(&opt)
	hardLimit := opt.fileSizeLimit * HardLimitMultiplier

	var data T
	var mtime time.Time
	var files []string

	var lastErr error
	for {
		select {
		default:
		case <-ctx.Done():
			return nil, fmt.Errorf(
				"filewatcher: context canceled while waiting for file(s) under %q to load: %w, last err: %w",
				path,
				ctx.Err(),
				lastErr,
			)
		}

		var err error
		data, mtime, files, err = openAndParse(path, parser, opt.fileSizeLimit, hardLimit)
		if errors.Is(err, fs.ErrNotExist) {
			lastErr = err
			time.Sleep(opt.initialReadInterval)
			continue
		}
		if err != nil {
			return nil, err
		}
		break
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	for _, path := range files {
		// Evaluate any symlinks before adding to work around https://github.com/fsnotify/fsnotify/issues/652
		real, err := filepath.EvalSymlinks(path)
		if err != nil {
			slog.ErrorContext(ctx, "filewatcher: failed to evaluate symlinks; using original name", "err", err, "path", path)
			real = path
		}
		if err := watcher.Add(real); err != nil {
			return nil, fmt.Errorf(
				"filewatcher: failed to watch %q: %w",
				path,
				err,
			)
		}
	}

	res := new(Result[T])
	res.data.Store(&dataAt[T]{
		data:  data,
		mtime: mtime,
	})
	res.ctx, res.cancel = context.WithCancel(context.WithoutCancel(ctx))

	go res.watcherLoop(
		watcher,
		path,
		parser,
		opt.fileSizeLimit,
		hardLimit,
		opt.pollingInterval,
		opt.fsEventsDelay,
	)

	return res, nil
}
