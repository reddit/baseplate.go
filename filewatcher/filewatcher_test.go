package filewatcher_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/reddit/baseplate.go/filewatcher"
	"github.com/reddit/baseplate.go/log"
)

const fsEventsDelayForTests = 10 * time.Millisecond

func parser(f io.Reader) (interface{}, error) {
	return io.ReadAll(f)
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

func compareBytesData(t *testing.T, data interface{}, expected []byte) {
	t.Helper()

	if data == nil {
		t.Error("data is nil")
		return
	}
	b, ok := (data).([]byte)
	if !ok {
		t.Errorf("data is not of type *[]byte, actual value: %#v", data)
		return
	}
	if string(b) != string(expected) {
		t.Errorf("*data expected to be %q, got %q", expected, b)
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
			interval := fsEventsDelayForTests
			backupInitialReadInterval := filewatcher.InitialReadInterval
			t.Cleanup(func() {
				filewatcher.InitialReadInterval = backupInitialReadInterval
			})
			filewatcher.InitialReadInterval = interval
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
				filewatcher.Config{
					Path:            path,
					Parser:          parser,
					Logger:          log.TestWrapper(t),
					PollingInterval: c.interval,
					FSEventsDelay:   fsEventsDelayForTests,
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(data.Stop)
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
	interval := fsEventsDelayForTests
	backupInitialReadInterval := filewatcher.InitialReadInterval
	t.Cleanup(func() {
		filewatcher.InitialReadInterval = backupInitialReadInterval
	})
	filewatcher.InitialReadInterval = interval
	round := interval * 20
	timeout := round * 4

	dir := t.TempDir()
	path := filepath.Join(dir, "foo")

	before := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)
	_, err := filewatcher.New(
		ctx,
		filewatcher.Config{
			Path:          path,
			Parser:        parser,
			Logger:        log.TestWrapper(t),
			FSEventsDelay: fsEventsDelayForTests,
		},
	)
	if err == nil {
		t.Error("Expected context cancellation error, got nil.")
	}
	duration := time.Since(before)
	if duration.Round(round) > timeout.Round(round) {
		t.Errorf("Timeout took %v instead of %v", duration, timeout)
	} else {
		t.Logf("Timeout took %v, set at %v", duration, timeout)
	}
}

func TestFileWatcherRename(t *testing.T) {
	interval := fsEventsDelayForTests
	backupInitialReadInterval := filewatcher.InitialReadInterval
	t.Cleanup(func() {
		filewatcher.InitialReadInterval = backupInitialReadInterval
	})
	filewatcher.InitialReadInterval = interval
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
		filewatcher.Config{
			Path:            path,
			Parser:          parser,
			Logger:          log.TestWrapper(t),
			PollingInterval: writeDelay,
			FSEventsDelay:   fsEventsDelayForTests,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(data.Stop)
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
	errParser := errors.New("parser failed")
	var n int64
	parser := func(_ io.Reader) (interface{}, error) {
		// This parser implementation fails every other call
		value := atomic.AddInt64(&n, 1)
		if value%2 == 0 {
			return nil, errParser
		}
		return value, nil
	}
	var loggerCalled int64
	logger := func(_ context.Context, msg string) {
		atomic.StoreInt64(&loggerCalled, 1)
		t.Log(msg)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "foo")

	writeFile(t, path, nil)

	// Initial call to parser should return 1, nil
	data, err := filewatcher.New(
		context.Background(),
		filewatcher.Config{
			Path:            path,
			Parser:          parser,
			Logger:          logger,
			PollingInterval: -1, // disable polling as we need exact numbers of parser calls in this test
			FSEventsDelay:   fsEventsDelayForTests,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(data.Stop)
	expected := int64(1)
	value := data.Get().(int64)
	if value != expected {
		t.Errorf("data.Get().(int64) expected %d, got %d", expected, value)
	}

	// Next call to parser should return nil, err
	newpath := path + ".bar"
	writeFile(t, newpath, nil)
	if err := os.Rename(newpath, path); err != nil {
		t.Fatal(err)
	}
	// Give it some time to handle the file content change
	time.Sleep(500 * time.Millisecond)
	if atomic.LoadInt64(&loggerCalled) == 0 {
		t.Error("Expected logger being called")
	}
	value = data.Get().(int64)
	if value != expected {
		t.Errorf("data.Get().(int64) expected %d, got %d", expected, value)
	}

	// Next call to parser should return 3, nil
	writeFile(t, newpath, nil)
	if err := os.Rename(newpath, path); err != nil {
		t.Fatal(err)
	}
	// Give it some time to handle the file content change
	time.Sleep(500 * time.Millisecond)
	expected = 3
	value = data.Get().(int64)
	if value != expected {
		t.Errorf("data.Get().(int64) expected %d, got %d", expected, value)
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
	root := t.TempDir()
	dir := filepath.Join(root, "dir")
	if err := os.Mkdir(dir, 0777); err != nil {
		t.Fatalf("Failed to create directory %q: %v", dir, err)
	}
	var parserCalled int64
	parser := filewatcher.DirParser(func(dir fs.FS) (any, error) {
		atomic.AddInt64(&parserCalled, 1)
		m := make(map[string]string)
		if err := fs.WalkDir(dir, ".", func(path string, de fs.DirEntry, err error) error {
			if err != nil {
				return nil
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
		filewatcher.Config{
			Path:            dir,
			Parser:          filewatcher.WrapDirParser(parser),
			Logger:          log.TestWrapper(t),
			PollingInterval: -1, // disable polling
			FSEventsDelay:   fsEventsDelayForTests,
		},
	)
	if err != nil {
		t.Fatalf("Failed to create filewatcher: %v", err)
	}
	t.Cleanup(data.Stop)

	got := data.Get().(map[string]string)
	if diff := cmp.Diff(got, content1); diff != "" {
		t.Errorf("unexpected result (-got, +want):\n%s", diff)
	}
	if got, want := atomic.LoadInt64(&parserCalled), int64(1); got != want {
		t.Errorf("Got %d parser called, want %d", got, want)
	}

	content2 := map[string]string{
		"foo/buzz": "hello, world!",
		"bar":      "bye, world!",
	}
	updateDirWithContents(t, dir, content2)
	time.Sleep(fsEventsDelayForTests * 5)
	got = data.Get().(map[string]string)
	if diff := cmp.Diff(got, content2); diff != "" {
		t.Errorf("unexpected result (-got, +want):\n%s", diff)
	}
	if got, want := atomic.LoadInt64(&parserCalled), int64(2); got != want {
		t.Errorf("Got %d parser called, want %d", got, want)
	}

	content3 := map[string]string{
		"foo": "hello, world!",
		"bar": "bye, world!",
	}
	updateDirWithContents(t, dir, content3)
	time.Sleep(fsEventsDelayForTests * 5)
	got = data.Get().(map[string]string)
	if diff := cmp.Diff(got, content3); diff != "" {
		t.Errorf("unexpected result (-got, +want):\n%s", diff)
	}
	if got, want := atomic.LoadInt64(&parserCalled), int64(3); got != want {
		t.Errorf("Got %d parser called, want %d", got, want)
	}
}

func limitedParser(t *testing.T, expectedSize int64) filewatcher.Parser {
	return func(f io.Reader) (interface{}, error) {
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

type logWrapper struct {
	called int64
}

func (w *logWrapper) wrapper(tb testing.TB) log.Wrapper {
	return func(_ context.Context, msg string) {
		tb.Helper()
		tb.Logf("logger called with msg: %q", msg)
		atomic.AddInt64(&w.called, 1)
	}
}

func (w *logWrapper) getCalled() int64 {
	return atomic.LoadInt64(&w.called)
}

func TestParserSizeLimit(t *testing.T) {
	interval := fsEventsDelayForTests
	backupInitialReadInterval := filewatcher.InitialReadInterval
	t.Cleanup(func() {
		filewatcher.InitialReadInterval = backupInitialReadInterval
	})
	filewatcher.InitialReadInterval = interval
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
	var wrapper logWrapper
	data, err := filewatcher.New(
		ctx,
		filewatcher.Config{
			Path:            path,
			Parser:          limitedParser(t, size),
			Logger:          wrapper.wrapper(t),
			MaxFileSize:     limit,
			PollingInterval: writeDelay,
			FSEventsDelay:   fsEventsDelayForTests,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(data.Stop)
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
	if called := wrapper.getCalled(); called < expectedCalledMin {
		t.Errorf("Expected log.Wrapper to be called at least %d times, actual %d", expectedCalledMin, called)
	}
}

func TestMockFileWatcher(t *testing.T) {
	t.Parallel()

	const (
		foo = "foo"
		bar = "bar"
	)

	r := strings.NewReader(foo)
	fw, err := filewatcher.NewMockFilewatcher(r, func(r io.Reader) (interface{}, error) {
		var buf bytes.Buffer
		_, err := io.Copy(&buf, r)
		if err != nil {
			return "", err
		}
		return buf.String(), nil
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Run(
		"get",
		func(t *testing.T) {
			data, ok := fw.Get().(string)
			if !ok {
				t.Fatalf("%#v is not a string", data)
			}

			if strings.Compare(data, foo) != 0 {
				t.Fatalf("%q does not match %q", data, foo)
			}
		},
	)

	t.Run(
		"update",
		func(t *testing.T) {
			if err := fw.Update(strings.NewReader(bar)); err != nil {
				t.Fatal(err)
			}

			data, ok := fw.Get().(string)
			if !ok {
				t.Fatalf("%#v is not a string", data)
			}

			if strings.Compare(data, bar) != 0 {
				t.Fatalf("%q does not match %q", data, foo)
			}
		},
	)

	t.Run(
		"errors",
		func(t *testing.T) {
			t.Run(
				"NewMockFilewatcher",
				func(t *testing.T) {
					if _, err := filewatcher.NewMockFilewatcher(r, func(r io.Reader) (interface{}, error) {
						return "", errors.New("test")
					}); err == nil {
						t.Fatal("expected an error, got nil")
					}
				},
			)

			t.Run(
				"update",
				func(t *testing.T) {
					fw, err := filewatcher.NewMockFilewatcher(r, func(r io.Reader) (interface{}, error) {
						var buf bytes.Buffer
						_, err := io.Copy(&buf, r)
						if err != nil {
							return "", err
						}
						data := buf.String()
						if strings.Compare(data, bar) == 0 {
							return "", errors.New("test")
						}
						return data, nil
					})
					if err != nil {
						t.Fatal(err)
					}

					if err := fw.Update(strings.NewReader(bar)); err == nil {
						t.Fatal("expected an error, got nil")
					}
				},
			)
		},
	)
}
