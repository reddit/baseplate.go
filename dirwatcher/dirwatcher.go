package dirwatcher

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

// DirWatcher loads and parses data from a file and watches for changes to that
// file in order to refresh it's stored data.
type DirWatcher interface {
	// Get returns the latest, parsed data from the DirWatcher.
	Get() interface{}

	// Stop stops the DirWatcher.
	//
	// After Stop is called you won't get any updates on the file content,
	// but you can still call Get to get the last content before stopping.
	//
	// It's OK to call Stop multiple times.
	// Calls after the first one are essentially no-op.
	Stop()
}

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
	parser filewatcher.Parser,
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
			logger.Log(context.Background(), "dirwatcher: watcher error: "+err.Error())

		case ev := <-watcher.Events:
			if filepath.Base(ev.Name) != file {
				continue
			}

			isDir, err := isDirectory(path)
			if err != nil {
				logger.Log(context.Background(), "dirwatcher: watcher error: "+err.Error())
			}

			switch ev.Op {
			default:
				// Ignore uninterested events.
			case fsnotify.Create: // add to watcher, parse if file
				// Wrap with an anonymous function to make sure that defer works.
				func() {
					if isDir {
						watcher.Add(path)
					} else {
						f, err := limitopen.OpenWithLimit(path, softLimit, hardLimit)
						if err != nil {
							logger.Log(context.Background(), "dirwatcher: I/O error: "+err.Error())
							return
						}
						defer f.Close()
						d, err := parser(f)
						if err != nil {
							logger.Log(context.Background(), "dirwatcher: parser error: "+err.Error())
						} else {
							// folder := r.data.Load().(Folder)
							// folder.Files[path] = d
							// r.data.Store(folder) //merge?
							r.data.Store(d)
						}
					}
				}()
			case fsnotify.Rename, fsnotify.Remove: // remove from watcher
				// Wrap with an anonymous function to make sure that defer works.
				func() {
					if isDir {
						watcher.Remove(path)
					} else {
						// remove data related to path?
					}
				}()
			case fsnotify.Write, fsnotify.Chmod: //parse
				// Wrap with an anonymous function to make sure that defer works.
				func() {
					if isDir {
						// do nothing
					} else {
						f, err := limitopen.OpenWithLimit(path, softLimit, hardLimit)
						if err != nil {
							logger.Log(context.Background(), "dirwatcher: I/O error: "+err.Error())
							return
						}
						defer f.Close()
						d, err := parser(f)
						if err != nil {
							logger.Log(context.Background(), "dirwatcher: parser error: "+err.Error())
						} else {
							// folder := r.data.Load().(Folder)
							// folder.Files[path] = d
							// r.data.Store(folder) //merge?
							r.data.Store(d)
						}
					}
				}()
			}
		}
	}
}

var (
	_ DirWatcher = (*Result)(nil)
)

// Folder is a construct to sort data parsed from a dirwatcher by its file path
type Folder struct {
	Files map[string]interface{}
}

// func (folder *Folder) AddFile(path string, file interface{}) error {
// 	return nil
// }

// func (folder *Folder) RemoveFile(path string) error {
// 	return nil
// }

// New initializes a dirwatcher designed for recursivly
// looking through a directory instead of a file
func New(ctx context.Context, cfg filewatcher.Config) (*Result, error) {
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
	// res.data.Store(Folder{
	// 	Files: make(map[string]interface{}),
	// })
	// Need to walk recursively because the watcher
	// doesnt support recursion by itself
	secretPath := filepath.Clean(cfg.Path)
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
				return fmt.Errorf("dirwatcher: context cancelled while waiting for file under %q to load. %w", cfg.Path, ctx.Err())
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
			// folder := res.data.Load().(Folder)
			// folder.Files[path] = d
			// res.data.Store(folder) //merge?

			// data := res.data.Load()
			// if err := mergo.MergeWithOverwrite(&data, d); err != nil {
			// 	watcher.Close()
			// 	return err
			// }
			// res.data.Store(data)

			res.data.Store(d)

			f.Close()

			return nil
		}()
	})
	if err != nil {
		return nil, err
	}

	res.ctx, res.cancel = context.WithCancel(context.Background())
	go res.watcherLoop(watcher, cfg.Path, cfg.Parser, limit, hardLimit, cfg.Logger)

	return res, nil
}

// NewMockDirwatcher returns a pointer to a new MockDirWatcher object
// initialized with the given io.Reader and Parser.
func NewMockDirwatcher(r io.Reader, parser filewatcher.Parser) (*MockDirWatcher, error) {
	fw := &MockDirWatcher{parser: parser}
	if err := fw.Update(r); err != nil {
		return nil, err
	}
	return fw, nil
}

// MockDirWatcher is an implementation of DirWatcher that does not actually read
// from a file, it simply returns the data given to it when it was initialized
// with NewMockDirwatcher. It provides an additional Update method that allows
// you to update this data after it has been created.
type MockDirWatcher struct {
	data   atomic.Value
	parser filewatcher.Parser
}

// Update updates the data of the MockDirWatcher using the given io.Reader and
// the Parser used to initialize the file watcher.
//
// This method is not threadsafe.
func (fw *MockDirWatcher) Update(r io.Reader) error {
	data, err := fw.parser(r)
	if err != nil {
		return err
	}
	fw.data.Store(data)
	return nil
}

// Get returns the parsed data.
func (fw *MockDirWatcher) Get() interface{} {
	return fw.data.Load()
}

// Stop is a no-op.
func (fw *MockDirWatcher) Stop() {}

var _ DirWatcher = (*MockDirWatcher)(nil)

func isDirectory(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	return fileInfo.IsDir(), err
}
