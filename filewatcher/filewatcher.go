package filewatcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"gopkg.in/fsnotify.v1"

	"github.com/reddit/baseplate.go/internal/limitopen"
	"github.com/reddit/baseplate.go/log"
)

// DefaultPollingInterval is the default PollingInterval used when creating a
// new file watcher.
const DefaultPollingInterval = 30 * time.Second

// FileWatcher loads and parses data from a file and watches for changes to that
// file in order to refresh it's stored data.
type FileWatcher interface {
	// Get returns the latest, parsed data from the FileWatcher.
	Get() interface{}

	// Close stops the FileWatcher.
	//
	// After Close is called you won't get any updates on the file content,
	// but you can still call Get to get the last content before stopping.
	//
	// It's OK to call Close multiple times.
	// Calls after the first one are essentially no-op.
	Close()
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

// Result is the return type of New. Use Get function to get the actual data.
type Result struct {
	data atomic.Value

	ctx    context.Context
	cancel context.CancelFunc
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

// Close stops the file watcher.
//
// After Close is called you won't get any updates on the file content,
// but you can still call Get to get the last content before stopping.
//
// It's OK to call Close multiple times.
// Calls after the first one are essentially no-op.
func (r *Result) Close() {
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
) {
	forceReload := func(mtime time.Time) {
		f, err := limitopen.OpenWithLimit(path, softLimit, hardLimit)
		if err != nil {
			logger.Log(context.Background(), "filewatcher: I/O error: "+err.Error())
			return
		}
		defer f.Close()
		d, err := parser(f)
		if err != nil {
			logger.Log(context.Background(), "filewatcher: parser error: "+err.Error())
		} else {
			r.data.Store(&atomicData{
				data:  d,
				mtime: mtime,
			})
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

	file := filepath.Base(path)
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

			if filepath.Base(ev.Name) != file {
				continue
			}

			switch ev.Op {
			default:
				// Ignore uninterested events.
			case fsnotify.Create, fsnotify.Write:
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

	var f io.ReadCloser
	var mtime time.Time

	for {
		select {
		default:
		case <-ctx.Done():
			return nil, fmt.Errorf("filewatcher: context cancelled while waiting for file under %q to load. %w", cfg.Path, ctx.Err())
		}

		var err error
		f, err = limitopen.OpenWithLimit(cfg.Path, limit, hardLimit)
		if errors.Is(err, os.ErrNotExist) {
			time.Sleep(InitialReadInterval)
			continue
		}
		if err != nil {
			return nil, err
		}
		defer f.Close()

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
	err = watcher.Add(filepath.Dir(cfg.Path))
	if err != nil {
		return nil, err
	}

	var d interface{}
	d, err = cfg.Parser(f)
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
	go res.watcherLoop(
		watcher,
		cfg.Path,
		cfg.Parser,
		limit,
		hardLimit,
		cfg.Logger,
		cfg.PollingInterval,
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

// Close is a no-op.
func (fw *MockFileWatcher) Close() {}

var _ FileWatcher = (*MockFileWatcher)(nil)
