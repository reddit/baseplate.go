package filewatcher

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"gopkg.in/fsnotify.v1"

	"github.com/reddit/baseplate.go/log"
)

// InitialReadInterval is the interval to keep retrying to open the file when
// creating a new file watcher, when the file was not initially available.
//
// It's intentionally defined as a variable instead of constant, so that the
// caller can tweak its value when needed.
var InitialReadInterval = time.Second / 2

// DefaultMaxFileSize is the default MaxFileSize used when it's <= 0.
//
// It's 1MiB, which is following the size limit of Apache ZooKeeper nodes.
const DefaultMaxFileSize = 1024 * 1024

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
	logger log.Wrapper,
) {
	file := filepath.Base(path)
	for {
		select {
		case <-r.ctx.Done():
			watcher.Close()
			return

		case err := <-watcher.Errors:
			log.FallbackWrapper(logger)("watcher error: " + err.Error())

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
					f, err := os.Open(path)
					if err != nil {
						log.FallbackWrapper(logger)("parser error: " + err.Error())
					}
					defer f.Close()
					d, err := parser(f)
					if err != nil {
						log.FallbackWrapper(logger)("parser error: " + err.Error())
					} else {
						r.data.Store(d)
					}
				}()
			}
		}
	}
}

// Config defines the config to be used in New function.
type Config struct {
	// The path to the file to be watched, required.
	Path string

	// The parser to parse the data load, required.
	Parser Parser

	// Optional. When non-nil, it will be used to log errors,
	// either returned by parser or by the underlying file system watcher.
	// Please note that this does not include errors returned by the first parser
	// call, which will be returned directly.
	Logger log.Wrapper

	// Optional. When <=0 DefaultMaxFileSize will be used instead.
	// This limits the size of the file that will be read into memory and sent to
	// the parser, to limit memory usage.
	// If the file content is larger than MaxFileSize,
	// we don't treat that as an error,
	// but the parser will only receive the first MaxFileSize bytes of content.
	//
	// The reasons behind the decision of not treating them as an error are:
	// 1. Some parsers only read up to what they need (example: json),
	//    so extra garbage after that won't cause any problems.
	// 2. For parsers that won't work when they only receive partial data,
	//    this will cause them to return an error,
	//    which will be logged and we won't update the parsed data,
	//    so it's essentially the same as treating those as errors.
	// 3. Getting the real size of the content without actually reading them could
	//    be tricky. Only send partial data to parsers is a more robust solution.
	MaxFileSize int64
}

func limitParser(parser Parser, limit int64) Parser {
	return func(f io.Reader) (interface{}, error) {
		return parser(io.LimitReader(f, limit))
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
func New(ctx context.Context, cfg Config) (*Result, error) {
	limit := cfg.MaxFileSize
	if limit <= 0 {
		limit = DefaultMaxFileSize
	}
	parser := limitParser(cfg.Parser, limit)

	var f *os.File

	for {
		select {
		default:
		case <-ctx.Done():
			return nil, fmt.Errorf("filewatcher: context cancelled while waiting for file to load. %w", ctx.Err())
		}

		var err error
		f, err = os.Open(cfg.Path)
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
	err = watcher.Add(filepath.Dir(cfg.Path))
	if err != nil {
		return nil, err
	}

	var d interface{}
	d, err = parser(f)
	if err != nil {
		watcher.Close()
		return nil, err
	}
	res := &Result{}
	res.data.Store(d)
	res.ctx, res.cancel = context.WithCancel(context.Background())

	go res.watcherLoop(watcher, cfg.Path, parser, cfg.Logger)

	return res, nil
}
