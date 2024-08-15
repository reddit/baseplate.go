package filewatcher

import (
	"context"
	"io"
	"io/fs"
	"time"

	v2 "github.com/reddit/baseplate.go/filewatcher/v2"
	"github.com/reddit/baseplate.go/filewatcher/v2/fwtest"
	"github.com/reddit/baseplate.go/log"
)

// DefaultFSEventsDelay is the default FSEventsDelay used when creating a new
// FileWatcher.
const DefaultFSEventsDelay = v2.DefaultFSEventsDelay

// DefaultPollingInterval is the default PollingInterval used when creating a
// new FileWatcher.
const DefaultPollingInterval = v2.DefaultPollingInterval

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
var InitialReadInterval = v2.DefaultInitialReadInterval

// DefaultMaxFileSize is the default MaxFileSize used when it's <= 0.
//
// It's 10 MiB, with hard limit multiplier of 10.
const (
	DefaultMaxFileSize  = v2.DefaultMaxFileSize
	HardLimitMultiplier = v2.HardLimitMultiplier
)

// A Parser is a callback function to be called when a watched file has its
// content changed, or is read for the first time.
//
// Please note that Parser should always return the consistent type.
// Inconsistent type will cause panic, as does returning nil data and nil error.
type Parser = v2.Parser[any]

// A DirParser is a callback function that will be called when the watched
// directory has its content changed or is read for the first time.
//
// Please note that a DirParser must return a consistent type.
// Inconsistent types will cause a panic,
// as does returning nil data and nil error.
//
// Use WrapDirParser to wrap it into a Parser to be used with FileWatcher.
type DirParser = v2.DirParser[any]

// WrapDirParser wraps a DirParser for a directory to a Parser.
//
// When using FileWatcher to watch a directory instead of a single file,
// you MUST use WrapDirParser instead of any other Parser implementations.
func WrapDirParser(dp DirParser) Parser {
	return v2.WrapDirParser(dp)
}

// Result is the return type of New. Use Get function to get the actual data.
type Result struct {
	result *v2.Result[any]
}

// Get returns the latest parsed data from the FileWatcher.
//
// Although the type is any,
// it's guaranteed to be whatever actual type is implemented inside Parser.
func (r *Result) Get() any {
	return r.result.Get()
}

// Stop stops the FileWatcher.
//
// After Stop is called you won't get any updates on the file content,
// but you can still call Get to get the last content before stopping.
//
// It's OK to call Stop multiple times.
// Calls after the first one are essentially no-op.
func (r *Result) Stop() {
	r.result.Close()
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
	//
	// Deprecated: Errors will be logged via slog at error level instead.
	Logger log.Wrapper `yaml:"logger,omitempty"`

	// Optional. When <=0 DefaultMaxFileSize will be used instead.
	//
	// This is completely ignored when DirParser is used.
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
	opts := []v2.Option{
		v2.WithInitialReadInterval(InitialReadInterval),
	}
	if cfg.MaxFileSize > 0 {
		opts = append(opts, v2.WithFileSizeLimit(cfg.MaxFileSize))
	}
	if cfg.PollingInterval != 0 {
		opts = append(opts, v2.WithPollingInterval(cfg.PollingInterval))
	}
	if cfg.FSEventsDelay > 0 {
		opts = append(opts, v2.WithFSEventsDelay(cfg.FSEventsDelay))
	}
	result, err := v2.New(ctx, cfg.Path, cfg.Parser, opts...)
	if err != nil {
		return nil, err
	}
	return &Result{result: result}, nil
}

// MockFileWatcher is an implementation of FileWatcher that does not actually read
// from a file, it simply returns the data given to it when it was initialized
// with NewMockFilewatcher. It provides an additional Update method that allows
// you to update this data after it has been created.
type MockFileWatcher struct {
	fake *fwtest.FakeFileWatcher[any]
}

// NewMockFilewatcher returns a pointer to a new MockFileWatcher object
// initialized with the given io.Reader and Parser.
func NewMockFilewatcher(r io.Reader, parser Parser) (*MockFileWatcher, error) {
	fake, err := fwtest.NewFakeFilewatcher(r, parser)
	if err != nil {
		return nil, err
	}
	return &MockFileWatcher{fake: fake}, nil
}

// Update updates the data of the MockFileWatcher using the given io.Reader and
// the Parser used to initialize the FileWatcher.
//
// This method is not threadsafe.
func (fw *MockFileWatcher) Update(r io.Reader) error {
	return fw.fake.Update(r)
}

// Get returns the parsed data.
func (fw *MockFileWatcher) Get() any {
	return fw.fake.Get()
}

// Stop is a no-op.
func (fw *MockFileWatcher) Stop() {}

// MockDirWatcher is an implementation of FileWatcher for testing with watching
// directories.
type MockDirWatcher struct {
	fake *fwtest.FakeDirWatcher[any]
}

// NewMockDirWatcher creates a MockDirWatcher with the initial data and the
// given DirParser.
//
// It provides Update function to update the data after it's been created.
func NewMockDirWatcher(dir fs.FS, parser DirParser) (*MockDirWatcher, error) {
	fake, err := fwtest.NewFakeDirWatcher(dir, parser)
	if err != nil {
		return nil, err
	}
	return &MockDirWatcher{fake: fake}, nil
}

// Update updates the data stored in this MockDirWatcher.
func (dw *MockDirWatcher) Update(dir fs.FS) error {
	return dw.fake.Update(dir)
}

// Get implements FileWatcher by returning the last updated data.
func (dw *MockDirWatcher) Get() any {
	return dw.fake.Get()
}

// Stop is a no-op.
func (dw *MockDirWatcher) Stop() {}

var (
	_ FileWatcher = (*MockFileWatcher)(nil)
	_ FileWatcher = (*MockDirWatcher)(nil)
)
