package filewatcher

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
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
	data interface{}

	lock sync.Mutex

	ctx    context.Context
	cancel context.CancelFunc
}

// Get returns the latest parsed data from the file watcher.
//
// Although the type is interface{},
// it's guaranteed to be whatever actual type is implemented inside Parser.
func (r *Result) Get() interface{} {
	return r.data
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
	logger log.Wrapper,
) {
	file := filepath.Base(path)
	for {
		select {
		case <-r.ctx.Done():
			watcher.Close()
			return

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
				// Wrap with an anonymous function to make sure that defer works.
				func() {
					r.lock.Lock()
					defer r.lock.Unlock()

					f, err := os.Open(path)
					if err != nil {
						if logger != nil {
							logger("parser error: " + err.Error())
						}
					}
					defer f.Close()
					d, err := parser(f)
					if err != nil {
						if logger != nil {
							logger("parser error: " + err.Error())
						}
					}
					r.data = d
				}()
			}
		}
	}
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
func New(ctx context.Context, path string, parser Parser, logger log.Wrapper) (*Result, error) {
	var f *os.File

	for {
		select {
		default:
		case <-ctx.Done():
			return nil, ctx.Err()
		}

		var err error
		f, err = os.Open(path)
		if os.IsNotExist(err) {
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
	err = watcher.Add(filepath.Dir(path))
	if err != nil {
		return nil, err
	}

	var d interface{}
	d, err = parser(f)
	if err != nil {
		watcher.Close()
		return nil, err
	}
	res := &Result{
		data: d,
	}
	res.ctx, res.cancel = context.WithCancel(context.Background())

	go res.watcherLoop(watcher, path, parser, logger)

	return res, nil
}
