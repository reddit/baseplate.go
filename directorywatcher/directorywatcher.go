package directorywatcher

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync/atomic"

	"gopkg.in/fsnotify.v1"

	"github.com/reddit/baseplate.go/filewatcher"
	"github.com/reddit/baseplate.go/internal/limitopen"
	"github.com/reddit/baseplate.go/log"
)

// DirectoryWatcher loads and parses data from a directory and recursivelywatches for changes to that
// directory and in order to refresh it's stored data.
type DirectoryWatcher interface {
	// Get returns the latest, parsed data from the DirectoryWatcher.
	Get() interface{}

	// Stop stops the DirectoryWatcher.
	//
	// After Stop is called you won't get any updates on the directory content,
	// but you can still call Get to get the last content before stopping.
	//
	// It's OK to call Stop multiple times.
	// Calls after the first one are essentially no-op.
	Stop()
}

// AddFile is a type of function that should be ran to handle
// adding data after a file has been parsed
type AddFile func(d interface{}, file interface{}) (data interface{}, err error)

// RemoveFile is a type of function that should be ran to handle removing
// data after a file has been removed from the watcher
type RemoveFile func(d interface{}, path string) (data interface{}, err error)

// Result is the return type of New. Use Get function to get the actual data.
type Result struct {
	data atomic.Value

	ctx    context.Context
	cancel context.CancelFunc
}

// Config defines the config to be used in New function.
//
// Can be deserialized from YAML.
type Config struct {
	// The path to the directory to be watched, required.
	Path string `yaml:"path"`

	// The parser to parse the data load, required.
	Parser filewatcher.Parser

	Adder AddFile

	Remover RemoveFile

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
	parser filewatcher.Parser,
	add AddFile,
	remove RemoveFile,
	softLimit, hardLimit int64,
	logger log.Wrapper,
) {
	for {
		select {
		case <-r.ctx.Done():
			watcher.Close()
			return

		case err := <-watcher.Errors:
			logger.Log(context.Background(), "directorywatcher: watcher error: "+err.Error())

		case ev := <-watcher.Events:

			switch ev.Op {
			default:
				// do nothing.
			case fsnotify.Create, fsnotify.Write, fsnotify.Chmod:
				writeEvent(
					ev,
					watcher,
					parser,
					add,
					r,
					softLimit, hardLimit,
					logger,
				)
			case fsnotify.Rename, fsnotify.Remove:
				removeEvent(
					ev,
					remove,
					r,
					logger,
				)
			}
		}
	}
}

var (
	_ DirectoryWatcher = (*Result)(nil)
)

// New initializes a directorywatcher designed for recursivly
// looking through a directory instead of a file
func New(ctx context.Context, cfg Config) (*Result, error) {
	limit := cfg.MaxFileSize
	if limit <= 0 {
		limit = filewatcher.DefaultMaxFileSize
	}
	hardLimit := limit * filewatcher.HardLimitMultiplier

	var watcher *fsnotify.Watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	var d interface{}
	res := &Result{}

	// Need to walk recursively because the watcher
	// doesnt support recursion by itself
	dirPath := filepath.Clean(cfg.Path)

	isDir, err := isDirectory(dirPath)
	if !isDir {
		return nil, fmt.Errorf("directorywatcher: %q is not a directory", dirPath)
	} else if err != nil {
		return nil, err
	}

	err = filepath.WalkDir(dirPath, func(path string, info fs.DirEntry, err error) error {
		if info.IsDir() {
			return watcher.Add(path)
		}

		// Parse file if you find it
		return func() error {
			var f io.ReadCloser

			select {
			default:
			case <-ctx.Done():
				return fmt.Errorf("directorywatcher: context cancelled while waiting for file under %q to load. %w", cfg.Path, ctx.Err())
			}

			var err error
			f, err = limitopen.OpenWithLimit(path, limit, hardLimit)
			if err != nil {
				return err
			}
			d, err = cfg.Parser(f)
			if err != nil {
				watcher.Close()
				return err
			}
			data := res.data.Load()
			data, err = cfg.Adder(data, d)
			if err != nil {
				watcher.Close()
				return err
			}
			res.data.Store(data)

			f.Close()

			return nil
		}()
	})
	if err != nil {
		return nil, err
	}

	res.ctx, res.cancel = context.WithCancel(context.Background())
	go res.watcherLoop(watcher, cfg.Parser, cfg.Adder, cfg.Remover, limit, hardLimit, cfg.Logger)

	return res, nil
}

func isDirectory(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	return fileInfo.IsDir(), err
}

func writeEvent(
	ev fsnotify.Event,
	watcher *fsnotify.Watcher,
	parser filewatcher.Parser,
	add AddFile,
	r *Result,
	softLimit, hardLimit int64,
	logger log.Wrapper,
) {
	isDir, _ := isDirectory(ev.Name)
	if isDir && ev.Op == fsnotify.Create {
		watcher.Add(ev.Name)
	} else if isDir {
		// do nothing
	} else {
		f, err := limitopen.OpenWithLimit(ev.Name, softLimit, hardLimit)
		if err != nil {
			logger.Log(context.Background(), "directorywatcher: I/O error: "+err.Error())
			return
		}
		defer f.Close()
		d, err := parser(f)
		if err != nil {
			logger.Log(context.Background(), "directorywatcher: parser error: "+err.Error())
		} else {
			data := r.data.Load()
			data, err = add(data, d)
			if err != nil {
				logger.Log(context.Background(), "directorywatcher: add file error: "+err.Error())
				return
			}
			r.data.Store(data)
		}
	}
}

func removeEvent(
	ev fsnotify.Event,
	remove RemoveFile,
	r *Result,
	logger log.Wrapper,
) {
	data := r.data.Load()
	data, err := remove(data, ev.Name)
	if err != nil {
		logger.Log(context.Background(), "directorywatcher: remove file error: "+err.Error())
		return
	}
	r.data.Store(data)
}
