package filewatcher

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/fsnotify.v1"

	"github.snooguts.net/reddit/baseplate.go/log"
)

// InitialReadInterval is the interval to keep retrying to open the file when
// creating a new file watcher, when the file was not initially available.
//
// It's intentionally defined as a variable instead of constant, so that the
// caller can tweak its value when needed.
var InitialReadInterval = time.Second / 2

// A Parser is a callback function to be called when a watched file has its
// content changed, or is read for the first time.
type Parser func(f io.Reader) (data interface{}, err error)

// Result is the return type of New. Use Get function to get the actual data.
type Result struct {
	data *interface{}
}

// Get returns the latest parsed data from the file watcher.
//
// If New() didn't return an error and the Parser didn't return nil,
// Get is guaranteed to return non-nil.
//
// Although the type is interface{},
// it's guaranteed to be whatever actual type is implemented inside Parser.
func (r Result) Get() interface{} {
	if r.data == nil {
		return nil
	}
	return *r.data
}

// New creates a new file watcher.
//
// If the path is not available at the time of calling,
// it blocks until the file becomes available, or context is cancelled.
//
// When logger is non-nil, it will be used to log errors,
// either returned by parser or by the underlying file system watcher.
// Please note that this does not include errors returned by the first parser
// call, which will be returned directly.
func New(ctx context.Context, path string, parser Parser, logger log.Wrapper) (res Result, err error) {
	var f *os.File

	for {
		select {
		default:
		case <-ctx.Done():
			err = ctx.Err()
			return
		}

		f, err = os.Open(path)
		if os.IsNotExist(err) {
			err = nil
			time.Sleep(InitialReadInterval)
			continue
		}
		if err != nil {
			return
		}
		break
	}

	defer f.Close()

	var watcher *fsnotify.Watcher
	watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return
	}
	// Note: We need to watch the parent directory instead of the file itself,
	// because only watching the file won't give us CREATE events,
	// which will happen with atomic renames.
	err = watcher.Add(filepath.Dir(path))
	if err != nil {
		return
	}

	var d interface{}
	d, err = parser(f)
	if err != nil {
		watcher.Close()
		return
	}
	res.data = &d

	go watcherLoop(watcher, res.data, path, parser, logger)

	return
}

func watcherLoop(
	watcher *fsnotify.Watcher,
	data *interface{},
	path string,
	parser Parser,
	logger log.Wrapper,
) {
	file := filepath.Base(path)
	for {
		select {
		case err := <-watcher.Errors:
			if logger != nil {
				logger("watcher error: " + err.Error())
			}

		case ev := <-watcher.Events:
			if filepath.Base(ev.Name) != file {
				continue
			}

			switch ev.Op {
			default:
				// Ignore uninterested events.
			case fsnotify.Create, fsnotify.Write:
				d, err := func() (interface{}, error) {
					// Wrap with an anonymous function to make sure that defer works.
					f, err := os.Open(path)
					if err != nil {
						return nil, err
					}
					defer f.Close()
					return parser(f)
				}()

				if err != nil {
					if logger != nil {
						logger("parser error: " + err.Error())
					}
				} else {
					*data = d
				}
			}
		}
	}
}
