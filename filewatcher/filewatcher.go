package filewatcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"gopkg.in/fsnotify.v1"

	"github.com/reddit/baseplate.go/internal/limitopen"
	"github.com/reddit/baseplate.go/log"
)

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
// It's 1MiB, which is following the size limit of Apache ZooKeeper nodes.
const (
	DefaultMaxFileSize  = 1024 * 1024
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
// Although the type is interface{},
// it's guaranteed to be whatever actual type is implemented inside Parser.
func (r *Result) Get() interface{} {
	return r.data.Load()
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

func (r *Result) watcherLoop(
	watcher *fsnotify.Watcher,
	path string,
	parser Parser,
	softLimit, hardLimit int64,
	logger log.Wrapper,
) {
	file := filepath.Base(path)
	for {
		select {
		case <-r.ctx.Done():
			watcher.Close()
			return

		case err := <-watcher.Errors:
			logger.Log(context.Background(), "filewatcher: watcher error: "+err.Error())

		case ev := <-watcher.Events:
			if filepath.Base(ev.Name) != file {
				continue
			}

			switch ev.Op {
			default:
				// Ignore uninterested events.
			case fsnotify.Create, fsnotify.Write:
				// Wrap with an anonymous function to make sure that defer works.
				func() {
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
						r.data.Store(d)
					}
				}()
			}
		}
	}
}

func (r *Result) dirWatcherLoop(
	watcher *fsnotify.Watcher,
	path string,
	parser Parser,
	softLimit, hardLimit int64,
	logger log.Wrapper,
) {
	file := filepath.Base(path)
	for {
		select {
		case <-r.ctx.Done():
			watcher.Close()
			return

		case err := <-watcher.Errors:
			logger.Log(context.Background(), "filewatcher: watcher error: "+err.Error())

		case ev := <-watcher.Events:
			if filepath.Base(ev.Name) != file {
				continue
			}

			switch ev.Op {
			default:
				// Ignore uninterested events.
			case fsnotify.Create, fsnotify.Write:
				// Wrap with an anonymous function to make sure that defer works.
				func() {
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
						r.data.Store(d)
					}
				}()
			}
		}
	}
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
	// the violation will be reported via log.ErrorWithSentry,
	// but it does not stop the normal parsing process.
	//
	// If the hard limit is violated,
	// The loading of the file will fail immediately.
	MaxFileSize int64 `yaml:"maxFileSize"`
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
		break
	}

	defer f.Close()

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
	res := &Result{}
	res.data.Store(d)
	res.ctx, res.cancel = context.WithCancel(context.Background())

	go res.watcherLoop(watcher, cfg.Path, cfg.Parser, limit, hardLimit, cfg.Logger)

	return res, nil
}

// NewDirWatcher initializes a filewatcher designed for recursivly
// looking through a directory instead of a file
func NewDirWatcher(ctx context.Context, cfg Config) (*Result, error) {
	limit := cfg.MaxFileSize
	if limit <= 0 {
		limit = DefaultMaxFileSize
	}
	hardLimit := limit * HardLimitMultiplier

	var watcher *fsnotify.Watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	var d interface{}
	res := &Result{}
	// Need to walk recursively because the watcher
	// doesnt support recursion by itself
	secretPath := filepath.Clean(cfg.Path)
	files, _ := os.ReadDir(cfg.Path)
	for _, file := range files {
		fmt.Println(file.Name())
		fmt.Println(file.IsDir())
		fmt.Println(file.Type())

	}
	fmt.Println("yo!!!!!!!!!!!!!!!!!!!!!!!!!")
	fmt.Println(files)
	err = filepath.WalkDir(secretPath, func(path string, info fs.DirEntry, err error) error {
		if info.IsDir() {
			return watcher.Add(path)
		}

		// Parse file if you find it
		return func() error {
			var f io.ReadCloser

			select {
			default:
			case <-ctx.Done():
				return fmt.Errorf("filewatcher: context cancelled while waiting for file under %q to load. %w", cfg.Path, ctx.Err())
			}

			var err error
			f, err = limitopen.OpenWithLimit(path, limit, hardLimit)
			if err != nil {
				return err
			}
			if path != secretPath {
				fmt.Println("yo############################")
				fmt.Println(path)
				fmt.Println(secretPath)
				d, err = cfg.Parser(f) // this fails, why is dir check false?
				if err != nil {
					watcher.Close()
					return err
				}
			}
			res.data.Store(d)

			f.Close()

			return nil
		}()
	})
	if err != nil {
		return nil, err
	}

	done := make(chan bool)
	go func() {
		for {
			select {
			// watch for events
			case event := <-watcher.Events:
				// if create , add to watcher
				// if remove or rename, remove from watcher
				// if write or chmod, parse
				watcher.Add(event.Name)
				fmt.Printf("EVENT! %#v %#v\n", event.Op.String(), event.Name)

			// watch for errors
			case err := <-watcher.Errors:
				fmt.Println("ERROR", err)
			}
		}
	}()
	<-done

	res.ctx, res.cancel = context.WithCancel(context.Background())
	// go res.dirWatcherLoop(watcher, cfg.Path, cfg.Parser, limit, hardLimit, cfg.Logger)

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

func isDirectory(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	return fileInfo.IsDir(), err
}
