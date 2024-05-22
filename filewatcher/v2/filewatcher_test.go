package filewatcher_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/reddit/baseplate.go/filewatcher/v2"
)

const fsEventsDelayForTests = 10 * time.Millisecond

func parser(f io.Reader) ([]byte, error) {
	return io.ReadAll(f)
}

// Fails the test when called with level >= error
type failSlogHandler struct {
	slog.Handler

	tb testing.TB
}

func (fsh failSlogHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level >= slog.LevelError {
		fsh.tb.Errorf("slog called at %v level with: %q", r.Level, r.Message)
	} else {
		fsh.tb.Logf("slog called at %v level with: %q", r.Level, r.Message)
	}
	return nil
}

// Counts the number of calls
type countingSlogHandler struct {
	slog.Handler

	tb    testing.TB
	count atomic.Int64
}

func (csh *countingSlogHandler) Handle(ctx context.Context, r slog.Record) error {
	csh.count.Add(1)
	if csh.tb != nil {
		csh.tb.Logf("slog called at %v level with: %q", r.Level, r.Message)
		return nil
	}
	return csh.Handler.Handle(ctx, r)
}

func swapSlog(tb testing.TB, logger *slog.Logger) {
	backupLogger := slog.Default()
	tb.Cleanup(func() {
		slog.SetDefault(backupLogger)
	})
	slog.SetDefault(logger)
}

// writeFile does atomic write/overwrite (write to a tmp file, then use rename
// to overwrite the desired path) instead of in-pleace write/overwrite
// (open/truncate open the file, write to it, close the file).
//
// filewatcher is designed to handle atomic writes/overwrites, not in-place
// ones. Doing in-place write will cause the filewatcher to be triggered twice
// (once the file is created/truncated, once when closing the file), which would
// cause some of the tests to fail flakily on CI.
func writeFile(tb testing.TB, path string, content []byte) {
	tb.Helper()

	tmpPath := filepath.Join(tb.TempDir(), "file")
	if err := os.WriteFile(tmpPath, content, 0644); err != nil {
		tb.Fatalf("Unable to write file: %v", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		tb.Fatalf("Unable to rename file: %v", err)
	}
}

func compareBytesData(t *testing.T, data []byte, expected []byte) {
	t.Helper()

	if string(data) != string(expected) {
		t.Errorf("*data expected to be %q, got %q", expected, data)
	}
}

func TestFileWatcher(t *testing.T) {
	for _, c := range []struct {
		label    string
		interval time.Duration
	}{
		{
			label:    "with-polling",
			interval: filewatcher.DefaultPollingInterval,
		},
		{
			label:    "no-polling",
			interval: -1,
		},
	} {
		t.Run(c.label, func(t *testing.T) {
			swapSlog(t, slog.New(failSlogHandler{
				tb:      t,
				Handler: slog.Default().Handler(),
			}))

			interval := fsEventsDelayForTests
			writeDelay := interval * 10
			timeout := writeDelay * 20

			payload1 := []byte("Hello, world!")
			payload2 := []byte("Bye, world!")
			payload3 := []byte("Hello, world, again!")

			dir := t.TempDir()
			path := filepath.Join(dir, "foo")

			// Delay writing the file
			go func() {
				time.Sleep(writeDelay)
				writeFile(t, path, payload1)
			}()

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			t.Cleanup(cancel)
			data, err := filewatcher.New(
				ctx,
				path,
				parser,
				filewatcher.WithPollingInterval(c.interval),
				filewatcher.WithFSEventsDelay(fsEventsDelayForTests),
				filewatcher.WithInitialReadInterval(interval),
			)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				data.Close()
			})
			compareBytesData(t, data.Get(), payload1)

			writeFile(t, path, payload2)
			// Give it some time to handle the file content change
			time.Sleep(500 * time.Millisecond)
			compareBytesData(t, data.Get(), payload2)

			writeFile(t, path, payload3)
			// Give it some time to handle the file content change
			time.Sleep(500 * time.Millisecond)
			compareBytesData(t, data.Get(), payload3)
		})
	}
}

func TestFileWatcherTimeout(t *testing.T) {
	swapSlog(t, slog.New(failSlogHandler{
		tb:      t,
		Handler: slog.Default().Handler(),
	}))

	interval := fsEventsDelayForTests
	round := interval * 20
	timeout := round * 4

	dir := t.TempDir()
	path := filepath.Join(dir, "foo")

	before := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)
	data, err := filewatcher.New(
		ctx,
		path,
		parser,
		filewatcher.WithFSEventsDelay(fsEventsDelayForTests),
		filewatcher.WithInitialReadInterval(interval),
	)
	if err == nil {
		t.Error("Expected context cancellation error, got nil.")
		t.Cleanup(func() {
			data.Close()
		})
	}
	duration := time.Since(before)
	if duration.Round(round) > timeout.Round(round) {
		t.Errorf("Timeout took %v instead of %v", duration, timeout)
	} else {
		t.Logf("Timeout took %v, set at %v", duration, timeout)
	}
}

func TestFileWatcherRename(t *testing.T) {
	swapSlog(t, slog.New(failSlogHandler{
		tb:      t,
		Handler: slog.Default().Handler(),
	}))

	interval := fsEventsDelayForTests
	writeDelay := interval * 10
	timeout := writeDelay * 20

	payload1 := []byte("Hello, world!")
	payload2 := []byte("Bye, world!")

	dir := t.TempDir()
	path := filepath.Join(dir, "foo")

	// Delay writing the file
	go func() {
		time.Sleep(writeDelay)
		writeFile(t, path, payload1)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)
	data, err := filewatcher.New(
		ctx,
		path,
		parser,
		filewatcher.WithPollingInterval(writeDelay),
		filewatcher.WithFSEventsDelay(fsEventsDelayForTests),
		filewatcher.WithInitialReadInterval(interval),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		data.Close()
	})
	compareBytesData(t, data.Get(), payload1)

	func() {
		newpath := path + ".bar"
		writeFile(t, newpath, payload2)
		if err := os.Rename(newpath, path); err != nil {
			t.Fatal(err)
		}
	}()
	// Give it some time to handle the file content change
	time.Sleep(writeDelay * 10)
	compareBytesData(t, data.Get(), payload2)
}

func TestParserFailure(t *testing.T) {
	counter := countingSlogHandler{
		Handler: slog.Default().Handler(),
		tb:      t,
	}
	swapSlog(t, slog.New(&counter))

	errParser := errors.New("parser failed")
	var n atomic.Int64
	parser := func(io.Reader) (int64, error) {
		// This parser implementation fails every other call
		value := n.Add(1)
		if value%2 == 0 {
			return 0, errParser
		}
		return value, nil
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "foo")

	writeFile(t, path, nil)

	// Initial call to parser should return 1, nil
	data, err := filewatcher.New(
		context.Background(),
		path,
		parser,
		filewatcher.WithPollingInterval(-1), // disable polling as we need exact numbers of parser calls in this test
		filewatcher.WithFSEventsDelay(fsEventsDelayForTests),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		data.Close()
	})
	expected := int64(1)
	if value := data.Get(); value != expected {
		t.Errorf("data.Get() expected %d, got %d", expected, value)
	}

	// Next call to parser should return nil, err
	newpath := path + ".bar"
	writeFile(t, newpath, nil)
	if err := os.Rename(newpath, path); err != nil {
		t.Fatal(err)
	}
	// Give it some time to handle the file content change
	time.Sleep(500 * time.Millisecond)
	if counter.count.Load() == 0 {
		t.Error("Expected logger being called")
	}
	if value := data.Get(); value != expected {
		t.Errorf("data.Get() expected %d, got %d", expected, value)
	}

	// Next call to parser should return 3, nil
	writeFile(t, newpath, nil)
	if err := os.Rename(newpath, path); err != nil {
		t.Fatal(err)
	}
	// Give it some time to handle the file content change
	time.Sleep(500 * time.Millisecond)
	if got, want := data.Get(), int64(3); got != want {
		t.Errorf("data.Get() got %d, want %d", got, want)
	}
}

func updateDirWithContents(tb testing.TB, dst string, contents map[string]string) {
	tb.Helper()

	root := tb.TempDir()
	dir := filepath.Join(root, "dir")

	if err := os.Mkdir(dir, 0777); err != nil {
		tb.Fatalf("Failed to create directory %q: %v", dir, err)
	}
	for p, content := range contents {
		path := filepath.Join(dir, p)
		parent := filepath.Dir(path)
		if err := os.Mkdir(parent, 0777); err != nil && !errors.Is(err, fs.ErrExist) {
			tb.Fatalf("Failed to create directory %q for %q: %v", parent, path, err)
		}
		if err := os.WriteFile(path, []byte(content), 0666); err != nil {
			tb.Fatalf("Failed to write file %q: %v", path, err)
		}
	}
	if err := os.RemoveAll(dst); err != nil && !errors.Is(err, fs.ErrNotExist) {
		tb.Fatalf("Failed to remove %q: %v", dst, err)
	}
	if err := os.Rename(dir, dst); err != nil {
		tb.Fatalf("Failed to rename from %q to %q: %v", dir, dst, err)
	}
}

func TestFileWatcherDir(t *testing.T) {
	swapSlog(t, slog.New(failSlogHandler{
		tb:      t,
		Handler: slog.Default().Handler(),
	}))

	root := t.TempDir()
	dir := filepath.Join(root, "dir")
	if err := os.Mkdir(dir, 0777); err != nil {
		t.Fatalf("Failed to create directory %q: %v", dir, err)
	}
	var parserCalled atomic.Int64
	parser := filewatcher.WrapDirParser(func(dir fs.FS) (map[string]string, error) {
		parserCalled.Add(1)
		m := make(map[string]string)
		if err := fs.WalkDir(dir, ".", func(path string, de fs.DirEntry, err error) error {
			if err != nil {
				return nil // skip to the next file
			}
			if de.IsDir() {
				return nil
			}
			f, err := dir.Open(path)
			if err != nil {
				return fmt.Errorf("failed to open %q: %w", path, err)
			}
			defer f.Close()
			content, err := io.ReadAll(f)
			if err != nil {
				return fmt.Errorf("failed to read %q: %w", path, err)
			}
			m[path] = string(content)
			return nil
		}); err != nil {
			return nil, err
		}
		return m, nil
	})

	content1 := map[string]string{
		"foo":      "hello, world!",
		"bar/fizz": "bye, world!",
	}
	updateDirWithContents(t, dir, content1)
	data, err := filewatcher.New(
		context.Background(),
		dir,
		parser,
		filewatcher.WithPollingInterval(-1), // disable polling
		filewatcher.WithFSEventsDelay(fsEventsDelayForTests),
	)
	if err != nil {
		t.Fatalf("Failed to create filewatcher: %v", err)
	}
	t.Cleanup(func() {
		data.Close()
	})

	if diff := cmp.Diff(data.Get(), content1); diff != "" {
		t.Errorf("unexpected result (-got, +want):\n%s", diff)
	}
	if got, want := parserCalled.Load(), int64(1); got != want {
		t.Errorf("Got %d parser called, want %d", got, want)
	}

	content2 := map[string]string{
		"foo/buzz": "hello, world!",
		"bar":      "bye, world!",
	}
	updateDirWithContents(t, dir, content2)
	time.Sleep(fsEventsDelayForTests * 5)
	if diff := cmp.Diff(data.Get(), content2); diff != "" {
		t.Errorf("unexpected result (-got, +want):\n%s", diff)
	}
	if got, want := parserCalled.Load(), int64(2); got != want {
		t.Errorf("Got %d parser called, want %d", got, want)
	}

	content3 := map[string]string{
		"foo": "hello, world!",
		"bar": "bye, world!",
	}
	updateDirWithContents(t, dir, content3)
	time.Sleep(fsEventsDelayForTests * 5)
	if diff := cmp.Diff(data.Get(), content3); diff != "" {
		t.Errorf("unexpected result (-got, +want):\n%s", diff)
	}
	if got, want := parserCalled.Load(), int64(3); got != want {
		t.Errorf("Got %d parser called, want %d", got, want)
	}
}

func limitedParser(t *testing.T, expectedSize int64) filewatcher.Parser[[]byte] {
	return func(f io.Reader) ([]byte, error) {
		var buf bytes.Buffer
		size, err := io.Copy(&buf, f)
		if err != nil {
			t.Error(err)
			return nil, err
		}
		if size != expectedSize {
			t.Errorf(
				"Expected size of %d, got %d, data %q",
				expectedSize,
				size,
				buf.Bytes(),
			)
		}
		return buf.Bytes(), nil
	}
}

func TestParserSizeLimit(t *testing.T) {
	counter := countingSlogHandler{
		Handler: slog.Default().Handler(),
		tb:      t,
	}
	swapSlog(t, slog.New(&counter))

	interval := fsEventsDelayForTests
	writeDelay := interval * 10
	timeout := writeDelay * 20

	const (
		content1 = "Hello, world!"
		content2 = "Bye bye, world!"
		limit    = int64(len(content1))
	)
	payload1 := bytes.Repeat([]byte(content1), filewatcher.HardLimitMultiplier)
	size := int64(len(payload1))
	expectedPayload := payload1
	payload2 := bytes.Repeat([]byte(content2), filewatcher.HardLimitMultiplier)

	dir := t.TempDir()
	path := filepath.Join(dir, "foo")

	// Delay writing the file
	go func() {
		time.Sleep(writeDelay)
		writeFile(t, path, payload1)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)
	data, err := filewatcher.New(
		ctx,
		path,
		limitedParser(t, size),
		filewatcher.WithFileSizeLimit(limit),
		filewatcher.WithPollingInterval(writeDelay),
		filewatcher.WithFSEventsDelay(fsEventsDelayForTests),
		filewatcher.WithInitialReadInterval(interval),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		data.Close()
	})
	compareBytesData(t, data.Get(), expectedPayload)

	writeFile(t, path, payload2)
	// Give it some time to handle the file content change
	time.Sleep(writeDelay * 10)
	// We expect the second parse would fail because of the data is beyond the
	// hard limit, so the data should still be expectedPayload
	compareBytesData(t, data.Get(), expectedPayload)
	// Since we expect the second parse would fail, we also expect the logger to
	// be called at least once.
	// The logger could be called twice because of reload triggered by polling.
	const expectedCalledMin = 1
	if called := counter.count.Load(); called < expectedCalledMin {
		t.Errorf("Expected log.Wrapper to be called at least %d times, actual %d", expectedCalledMin, called)
	}
}
