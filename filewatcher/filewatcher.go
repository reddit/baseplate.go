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

	"gopkg.in/fsnotify.v1"

	"github.com/reddit/baseplate.go/internal/limitopen"
	"github.com/reddit/baseplate.go/log"
)

// DefaultPollingInterval is the default PollingInterval used when creating a
// new file watcher.
const DefaultPollingInterval = 30 * time.Second

// DefaultParseDelay is the default time needed without an event to parse again. Used in
// directory watchers when many events can happen in a short period of time
const DefaultParseDelay = 200 * time.Millisecond

// FileWatcher loads and parses data from a file and watches for changes to that
// file in order to refresh it's stored data.
type FileWatcher interface {
	// Get returns the latest, parsed data from the FileWatcher.
	Get() interface{}

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
// creating a new file watcher, when the file was not initially available.
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
type Parser func(f io.Reader) (data interface{}, err error)

// DirParser is a callback function to be called when file(s) in a watched directory
// has been touched, or read for the first time.
// Should always return a constant type or will cause panic
type DirParser func(path string) (data interface{}, err error)

// DirParserWrapper wraps a DirParser so that it may be used as a Parser in a
// filewatcher
func DirParserWrapper(dp DirParser) Parser {
	return func(f io.Reader) (interface{}, error) {
		return dp(f.(dummyReader).path)
	}
}

// Result is the return type of New. Use Get function to get the actual data.
type Result struct {
	data atomic.Value

	ctx    context.Context
	cancel context.CancelFunc

	lock  sync.Mutex
	timer *time.Timer
}

// Get returns the latest parsed data from the file watcher.
//
// It might also call Parser if the file is changed from the latest parsed data.
//
// Although the type is interface{},
// it's guaranteed to be whatever actual type is implemented inside Parser.
func (r *Result) Get() interface{} {
	return r.data.Load().(*atomicData).data
}

// Stop stops the file watcher.
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
	watched []string,
	path string,
	parser Parser,
	softLimit, hardLimit int64,
	logger log.Wrapper,
	pollingInterval time.Duration,
	parseDelay time.Duration,
) {
	isDir, err := isDirectory(path)
	if err != nil {
		logger.Log(context.Background(), "filewatcher: isDirectory error: "+err.Error())
	}
	refreshWatched := func() {
		for _, p := range watched {
			watcher.Remove(p)
		}
		watched = []string{}

		parentDir := filepath.Dir(path)
		watcher.Add(parentDir)
		watched = append(watched, parentDir)

		err := filepath.WalkDir(path, func(p string, info fs.DirEntry, err error) error {
			watched = append(watched, p)
			watcher.Add(p)
			return nil
		})
		if err != nil {
			logger.Log(context.Background(), "filewatcher: refreshWatched error: "+err.Error())
			return
		}
	}
	forceReload := func(mtime time.Time) {
		var reader io.Reader
		if isDir {
			reader = dummyReader{
				path: path,
			}
		} else {
			f, err := limitopen.OpenWithLimit(path, softLimit, hardLimit)
			if err != nil {
				logger.Log(context.Background(), "filewatcher: I/O error: "+err.Error())
				return
			}
			defer f.Close()
			reader = f
		}
		parse := func() {
			d, err := parser(reader)
			if err != nil {
				logger.Log(context.Background(), "filewatcher: parser error: "+err.Error())
			} else {
				r.data.Store(&atomicData{
					data:  d,
					mtime: mtime,
				})
			}
		}
		if isDir {
			r.lock.Lock()
			defer r.lock.Unlock()
			if r.timer != nil {
				r.timer.Stop()
			}

			r.timer = time.AfterFunc(parseDelay, func() {
				refreshWatched()
				parse()
			})
		} else {
			parse()
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
			forceReload(mtime)
		}
	}

	var tickerChan <-chan time.Time
	if pollingInterval > 0 {
		ticker := time.NewTicker(pollingInterval)
		defer ticker.Stop()
		tickerChan = ticker.C
	}

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
			case fsnotify.Create, fsnotify.Write, fsnotify.Remove:
				if ev.Op == fsnotify.Remove && !isDir {
					continue
				}
				mtime, err := getMtime(path)
				if err != nil {
					logger.Log(context.Background(), fmt.Sprintf(
						"filewatcher: failed to get mtime for %q: %v",
						path,
						err,
					))
					continue
				}
				forceReload(mtime)
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
	data interface{}

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

	// Optional, the time directory watcher should wait for notifications before
	// it should parse again
	ParseDelay time.Duration `yaml:"parseDelay"`
}

// New creates a new file watcher.
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

	var reader io.Reader
	var mtime time.Time
	var watched []string

	for {
		select {
		default:
		case <-ctx.Done():
			return nil, fmt.Errorf("filewatcher: context cancelled while waiting for file under %q to load. %w", cfg.Path, ctx.Err())
		}

		var err error
		f, err := limitopen.OpenWithLimit(cfg.Path, limit, hardLimit)
		if errors.Is(err, os.ErrNotExist) {
			time.Sleep(InitialReadInterval)
			continue
		}
		if err != nil {
			return nil, err
		}
		defer f.Close()
		reader = f

		mtime, err = getMtime(cfg.Path)
		if err != nil {
			return nil, err
		}
		break
	}

	var watcher *fsnotify.Watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	// Note: We need to watch the parent directory instead of the file itself,
	// because only watching the file won't give us CREATE events,
	// which will happen with atomic renames.
	parentDir := filepath.Dir(cfg.Path)
	err = watcher.Add(parentDir)
	watched = append(watched, parentDir)

	if err != nil {
		return nil, err
	}
	isDir, err := isDirectory(cfg.Path)
	if err != nil {
		return nil, err
	}
	if isDir {
		reader = dummyReader{
			path: cfg.Path,
		}
		// Need to walk recursively because the watcher
		// doesnt support recursion by itself
		err := filepath.WalkDir(cfg.Path, func(p string, info fs.DirEntry, err error) error {
			watched = append(watched, p)
			watcher.Add(p)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("filewatcher.New: Error while walking directory '%s': %s", cfg.Path, err)
		}
	}

	var d interface{}
	d, err = cfg.Parser(reader)
	if err != nil {
		watcher.Close()
		return nil, err
	}
	res := new(Result)
	res.data.Store(&atomicData{
		data:  d,
		mtime: mtime,
	})
	res.ctx, res.cancel = context.WithCancel(context.Background())

	if cfg.PollingInterval == 0 {
		cfg.PollingInterval = DefaultPollingInterval
	}
	if cfg.ParseDelay == 0 {
		cfg.ParseDelay = DefaultParseDelay
	}
	go res.watcherLoop(
		watcher,
		watched,
		cfg.Path,
		cfg.Parser,
		limit,
		hardLimit,
		cfg.Logger,
		cfg.PollingInterval,
		cfg.ParseDelay,
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
// the Parser used to initialize the file watcher.
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
func (fw *MockFileWatcher) Get() interface{} {
	return fw.data.Load()
}

// Stop is a no-op.
func (fw *MockFileWatcher) Stop() {}

var _ FileWatcher = (*MockFileWatcher)(nil)

// dummyReader is a mock struct used to hold the path for directory watchers
type dummyReader struct {
	path string
}

func (drc dummyReader) Read(p []byte) (int, error) {
	return 0, fmt.Errorf("filewatcher: This operation is not supported for directories")
}

func isDirectory(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	isDir := fileInfo.IsDir()
	return isDir, nil
}
