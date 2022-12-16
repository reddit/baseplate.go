package filewatcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/reddit/baseplate.go/errorsbp"
	"github.com/reddit/baseplate.go/internal/limitopen"
	"github.com/reddit/baseplate.go/log"
)

// DefaultFSEventsDelay is the default FSEventsDelay used when creating a new
// FileWatcher.
const DefaultFSEventsDelay = 1 * time.Second

// DefaultPollingInterval is the default PollingInterval used when creating a
// new FileWatcher.
const DefaultPollingInterval = 30 * time.Second

// FileWatcher loads and parses data from a file or directory, and watches for
// changes in order to refresh its stored data.
type FileWatcher interface {
	// Get returns the latest, parsed data from the FileWatcher.
	Get() any

	// Stop stops the FileWatcher.
	//
	// After Stop is called you won't get any updates on the file content,
	// but you can still call Get to get the last content before stopping.
	//
	// It's OK to call Stop multiple times.
	// Calls after the first one are essentially no-op.
	Stop()
}

// InitialReadInterval is the interval to keep retrying to open the file when
// creating a new FileWatcher, when the file was not initially available.
//
// It's intentionally defined as a variable instead of constant, so that the
// caller can tweak its value when needed.
var InitialReadInterval = time.Second / 2

// DefaultMaxFileSize is the default MaxFileSize used when it's <= 0.
//
// It's 10 MiB, with hard limit multiplier of 10.
const (
	DefaultMaxFileSize  = 10 << 20
	HardLimitMultiplier = 10
)

// A Parser is a callback function to be called when a watched file has its
// content changed, or is read for the first time.
//
// Please note that Parser should always return the consistent type.
// Inconsistent type will cause panic, as does returning nil data and nil error.
type Parser func(f io.Reader) (data any, err error)

// A DirParser is a callback function that will be called when the watched
// directory has its content changed or is read for the first time.
//
// Please note that a DirParser must return a consistent type.
// Inconsistent types will cause a panic,
// as does returning nil data and nil error.
//
// Use WrapDirParser to wrap it into a Parser to be used with FileWatcher.
type DirParser func(dir fs.FS) (data any, err error)

// WrapDirParser wraps a DirParser for a directory to a Parser.
//
// When using FileWatcher to watch a directory instead of a single file,
// you MUST use WrapDirParser instead of any other Parser implementations.
func WrapDirParser(dp DirParser) Parser {
	return func(r io.Reader) (data any, err error) {
		path := string(r.(fakeDirectoryReader))
		dir := os.DirFS(path)
		return dp(dir)
	}
}

// Result is the return type of New. Use Get function to get the actual data.
type Result struct {
	data atomic.Value

	ctx    context.Context
	cancel context.CancelFunc
}

// Get returns the latest parsed data from the FileWatcher.
//
// Although the type is any,
// it's guaranteed to be whatever actual type is implemented inside Parser.
func (r *Result) Get() any {
	return r.data.Load().(*atomicData).data
}

// Stop stops the FileWatcher.
//
// After Stop is called you won't get any updates on the file content,
// but you can still call Get to get the last content before stopping.
//
// It's OK to call Stop multiple times.
// Calls after the first one are essentially no-op.
func (r *Result) Stop() {
	r.cancel()
}

func getMtime(path string) (time.Time, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return stat.ModTime(), nil
}

func (r *Result) watcherLoop(
	watcher *fsnotify.Watcher,
	path string,
	parser Parser,
	softLimit, hardLimit int64,
	logger log.Wrapper,
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
			logger.Log(context.Background(), err.Error())
		} else {
			r.data.Store(&atomicData{
				data:  d,
				mtime: mtime,
			})
			// remove all previously watched files
			for _, path := range watcher.WatchList() {
				watcher.Remove(path)
			}
			// then read all new files to watch
			for _, path := range files {
				if err := watcher.Add(path); err != nil {
					logger.Log(context.Background(), fmt.Sprintf(
						"filewatcher: failed to watch file %q: %v",
						path,
						err,
					))
				}
			}
		}
	}

	reload := func() {
		mtime, err := getMtime(path)
		if err != nil {
			logger.Log(context.Background(), fmt.Sprintf(
				"filewatcher: failed to get mtime for %q: %v",
				path,
				err,
			))
			return
		}
		if r.data.Load().(*atomicData).mtime.Before(mtime) {
			forceReload()
		}
	}

	var tickerChan <-chan time.Time
	if pollingInterval > 0 {
		ticker := time.NewTicker(pollingInterval)
		defer ticker.Stop()
		tickerChan = ticker.C
	}
	var timer *time.Timer
	for {
		select {
		case <-r.ctx.Done():
			watcher.Close()
			return

		case err := <-watcher.Errors:
			logger.Log(context.Background(), "filewatcher: watcher error: "+err.Error())

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

// The actual data held in Result.data.
type atomicData struct {
	// actual parsed data
	data any

	// other metadata
	mtime time.Time
}

var (
	_ FileWatcher = (*Result)(nil)
)

// Config defines the config to be used in New function.
//
// Can be deserialized from YAML.
type Config struct {
	// The path to the file to be watched, required.
	Path string `yaml:"path"`

	// The parser to parse the data load, required.
	Parser Parser

	// Optional. When non-nil, it will be used to log errors,
	// either returned by parser or by the underlying file system watcher.
	// Please note that this does not include errors returned by the first parser
	// call, which will be returned directly.
	Logger log.Wrapper `yaml:"logger"`

	// Optional. When <=0 DefaultMaxFileSize will be used instead.
	//
	// This is the soft limit,
	// we will also auto add a hard limit which is 10x (see HardLimitMultiplier)
	// of soft limit.
	//
	// If the soft limit is violated,
	// the violation will be reported via log.DefaultWrapper and prometheus
	// counter of limitopen_softlimit_violation_total,
	// but it does not stop the normal parsing process.
	//
	// If the hard limit is violated,
	// The loading of the file will fail immediately.
	MaxFileSize int64 `yaml:"maxFileSize"`

	// Optional, the interval to check file changes proactively.
	//
	// Default to DefaultPollingInterval.
	// To disable polling completely, set it to a negative value.
	//
	// Without polling filewatcher relies solely on fs events from the parent
	// directory. This works for most cases but will not work in the cases that
	// the parent directory will be remount upon change
	// (for example, k8s ConfigMap).
	PollingInterval time.Duration `yaml:"pollingInterval"`

	// Optional, the delay between receiving the fs events and actually reading
	// and parsing the changes.
	//
	// It's used to avoid short bursts of fs events (for example, when watching a
	// directory) causing reading and parsing repetively.
	//
	// Defaut to DefaultFSEventsDelay.
	FSEventsDelay time.Duration `yaml:"fsEventsDelay"`
}

type fakeDirectoryReader string

func (fakeDirectoryReader) Read([]byte) (int, error) {
	return 0, errors.New("filewatcher: you are most likely watching a directory without using DirParser")
}

func openAndParse(path string, parser Parser, limit, hardLimit int64) (data any, mtime time.Time, files []string, _ error) {
	stats, err := os.Stat(path)
	if err != nil {
		return nil, time.Time{}, nil, fmt.Errorf("filewatcher: i/o error: %w", err)
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
			return nil, time.Time{}, nil, fmt.Errorf("filewatcher: i/o error: %w", err)
		}
	} else {
		// file
		f, err := limitopen.OpenWithLimit(path, limit, hardLimit)
		if err != nil {
			return nil, time.Time{}, nil, fmt.Errorf("filewatcher: i/o error: %w", err)
		}
		defer f.Close()
		reader = f
	}

	d, err := parser(reader)
	if err != nil {
		return nil, time.Time{}, nil, fmt.Errorf("filewatcher: parser error: %w", err)
	}
	return d, mtime, files, nil
}

// New creates a new FileWatcher.
//
// If the path is not available at the time of calling,
// it blocks until the file becomes available, or context is cancelled,
// whichever comes first.
//
// When logger is non-nil, it will be used to log errors,
// either returned by parser or by the underlying file system watcher.
// Please note that this does not include errors returned by the first parser
// call, which will be returned directly.
func New(ctx context.Context, cfg Config) (*Result, error) {
	limit := cfg.MaxFileSize
	if limit <= 0 {
		limit = DefaultMaxFileSize
	}
	hardLimit := limit * HardLimitMultiplier

	var data any
	var mtime time.Time
	var files []string

	var lastErr error
	for {
		select {
		default:
		case <-ctx.Done():
			var batch errorsbp.Batch
			batch.Add(ctx.Err())
			batch.AddPrefix("last error", lastErr)
			return nil, fmt.Errorf("filewatcher: context canceled while waiting for file(s) under %q to load: %w", cfg.Path, batch.Compile())
		}

		var err error
		data, mtime, files, err = openAndParse(cfg.Path, cfg.Parser, limit, hardLimit)
		if errors.Is(err, fs.ErrNotExist) {
			lastErr = err
			time.Sleep(InitialReadInterval)
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
		if err := watcher.Add(path); err != nil {
			return nil, fmt.Errorf(
				"filewatcher: failed to watch %q: %w",
				path,
				err,
			)
		}
	}

	res := new(Result)
	res.data.Store(&atomicData{
		data:  data,
		mtime: mtime,
	})
	res.ctx, res.cancel = context.WithCancel(context.Background())

	if cfg.PollingInterval == 0 {
		cfg.PollingInterval = DefaultPollingInterval
	}
	if cfg.FSEventsDelay <= 0 {
		cfg.FSEventsDelay = DefaultFSEventsDelay
	}
	go res.watcherLoop(
		watcher,
		cfg.Path,
		cfg.Parser,
		limit,
		hardLimit,
		cfg.Logger,
		cfg.PollingInterval,
		cfg.FSEventsDelay,
	)

	return res, nil
}

// NewMockFilewatcher returns a pointer to a new MockFileWatcher object
// initialized with the given io.Reader and Parser.
func NewMockFilewatcher(r io.Reader, parser Parser) (*MockFileWatcher, error) {
	fw := &MockFileWatcher{parser: parser}
	if err := fw.Update(r); err != nil {
		return nil, err
	}
	return fw, nil
}

// MockFileWatcher is an implementation of FileWatcher that does not actually read
// from a file, it simply returns the data given to it when it was initialized
// with NewMockFilewatcher. It provides an additional Update method that allows
// you to update this data after it has been created.
type MockFileWatcher struct {
	data   atomic.Value
	parser Parser
}

// Update updates the data of the MockFileWatcher using the given io.Reader and
// the Parser used to initialize the FileWatcher.
//
// This method is not threadsafe.
func (fw *MockFileWatcher) Update(r io.Reader) error {
	data, err := fw.parser(r)
	if err != nil {
		return err
	}
	fw.data.Store(data)
	return nil
}

// Get returns the parsed data.
func (fw *MockFileWatcher) Get() any {
	return fw.data.Load()
}

// Stop is a no-op.
func (fw *MockFileWatcher) Stop() {}

// MockDirWatcher is an implementation of FileWatcher for testing with watching
// directories.
type MockDirWatcher struct {
	data   atomic.Value
	parser DirParser
}

// NewMockDirWatcher creates a MockDirWatcher with the initial data and the
// given DirParser.
//
// It provides Update function to update the data after it's been created.
func NewMockDirWatcher(dir fs.FS, parser DirParser) (*MockDirWatcher, error) {
	dw := &MockDirWatcher{parser: parser}
	if err := dw.Update(dir); err != nil {
		return nil, err
	}
	return dw, nil
}

// Update updates the data stored in this MockDirWatcher.
func (dw *MockDirWatcher) Update(dir fs.FS) error {
	data, err := dw.parser(dir)
	if err != nil {
		return err
	}
	dw.data.Store(data)
	return nil
}

// Get implements FileWatcher by returning the last updated data.
func (dw *MockDirWatcher) Get() any {
	return dw.data.Load()
}

// Stop is a no-op.
func (dw *MockDirWatcher) Stop() {}

var (
	_ FileWatcher = (*MockFileWatcher)(nil)
	_ FileWatcher = (*MockDirWatcher)(nil)
)
