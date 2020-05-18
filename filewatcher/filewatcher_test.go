package filewatcher_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/reddit/baseplate.go/filewatcher"
	"github.com/reddit/baseplate.go/log"
)

func parser(f io.Reader) (interface{}, error) {
	return ioutil.ReadAll(f)
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
	interval := time.Millisecond
	filewatcher.InitialReadInterval = interval
	writeDelay := interval * 10
	timeout := writeDelay * 20

	payload1 := []byte("Hello, world!")
	payload2 := []byte("Bye, world!")

	dir, err := ioutil.TempDir("", "filewatcher_test_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "foo")

	// Delay writing the file
	go func() {
		time.Sleep(writeDelay)
		f, err := os.Create(path)
		if err != nil {
			t.Error(err)
			return
		}
		defer f.Close()
		if _, err := f.Write(payload1); err != nil {
			t.Error(err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	data, err := filewatcher.New(
		ctx,
		filewatcher.Config{
			Path:   path,
			Parser: parser,
			Logger: log.TestWrapper(t),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer data.Stop()
	compareBytesData(t, data.Get(), payload1)

	func() {
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if _, err := f.Write(payload2); err != nil {
			t.Fatal(err)
		}
	}()
	// Give it some time to handle the file content change
	time.Sleep(time.Millisecond * 500)
	compareBytesData(t, data.Get(), payload2)
}

func TestFileWatcherTimeout(t *testing.T) {
	interval := time.Millisecond
	filewatcher.InitialReadInterval = interval
	round := interval * 20
	timeout := round * 4

	dir, err := ioutil.TempDir("", "filewatcher_test_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "foo")

	before := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_, err = filewatcher.New(
		ctx,
		filewatcher.Config{
			Path:   path,
			Parser: parser,
			Logger: log.TestWrapper(t),
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
	interval := time.Millisecond
	filewatcher.InitialReadInterval = interval
	writeDelay := interval * 10
	timeout := writeDelay * 20

	payload1 := []byte("Hello, world!")
	payload2 := []byte("Bye, world!")

	dir, err := ioutil.TempDir("", "filewatcher_test_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "foo")

	// Delay writing the file
	go func() {
		time.Sleep(writeDelay)
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if _, err := f.Write(payload1); err != nil {
			t.Error(err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	data, err := filewatcher.New(
		ctx,
		filewatcher.Config{
			Path:   path,
			Parser: parser,
			Logger: log.TestWrapper(t),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer data.Stop()
	compareBytesData(t, data.Get(), payload1)

	func() {
		newpath := path + ".bar"
		f, err := os.Create(newpath)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(newpath, path); err != nil {
				t.Fatal(err)
			}
		}()
		if _, err := f.Write(payload2); err != nil {
			t.Fatal(err)
		}
	}()
	// Give it some time to handle the file content change
	time.Sleep(interval * 10)
	compareBytesData(t, data.Get(), payload2)
}

func TestParserFailure(t *testing.T) {
	interval := time.Millisecond
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
	logger := func(msg string) {
		atomic.StoreInt64(&loggerCalled, 1)
		t.Log(msg)
	}

	dir, err := ioutil.TempDir("", "filewatcher_test_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "foo")

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	// Initial call to parser should return 1, nil
	data, err := filewatcher.New(
		context.Background(),
		filewatcher.Config{
			Path:   path,
			Parser: parser,
			Logger: logger,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer data.Stop()
	expected := int64(1)
	value := data.Get().(int64)
	if value != expected {
		t.Errorf("data.Get().(int64) expected %d, got %d", expected, value)
	}

	// Next call to parser should return nil, err
	newpath := path + ".bar"
	f, err = os.Create(newpath)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(newpath, path); err != nil {
		t.Fatal(err)
	}
	// Give it some time to handle the file content change
	time.Sleep(interval * 500)
	if atomic.LoadInt64(&loggerCalled) == 0 {
		t.Error("Expected logger being called")
	}
	value = data.Get().(int64)
	if value != expected {
		t.Errorf("data.Get().(int64) expected %d, got %d", expected, value)
	}

	// Next call to parser should return 3, nil
	f, err = os.Create(newpath)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(newpath, path); err != nil {
		t.Fatal(err)
	}
	// Give it some time to handle the file content change
	time.Sleep(interval * 500)
	expected = 3
	value = data.Get().(int64)
	if value != expected {
		t.Errorf("data.Get().(int64) expected %d, got %d", expected, value)
	}
}

func limitedParser(t *testing.T, expectedSize int) filewatcher.Parser {
	return func(f io.Reader) (interface{}, error) {
		buf, err := ioutil.ReadAll(f)
		if err != nil {
			t.Error(err)
			return nil, err
		}
		if len(buf) != expectedSize {
			t.Errorf(
				"Expected size of %d, got %d, data %q",
				expectedSize,
				len(buf),
				buf,
			)
		}
		return buf, nil
	}
}

func TestParserSizeLimit(t *testing.T) {
	interval := time.Millisecond
	filewatcher.InitialReadInterval = interval
	writeDelay := interval * 10
	timeout := writeDelay * 20

	const size = 3
	payload1 := []byte("Hello, world!")
	expectedPayload1 := payload1[:size]
	payload2 := []byte("Bye, world!")
	expectedPayload2 := payload2[:size]

	dir, err := ioutil.TempDir("", "filewatcher_test_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "foo")

	// Delay writing the file
	go func() {
		time.Sleep(writeDelay)
		f, err := os.Create(path)
		if err != nil {
			t.Error(err)
			return
		}
		defer f.Close()
		if _, err := f.Write(payload1); err != nil {
			t.Error(err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	data, err := filewatcher.New(
		ctx,
		filewatcher.Config{
			Path:        path,
			Parser:      limitedParser(t, size),
			Logger:      log.TestWrapper(t),
			MaxFileSize: size,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer data.Stop()
	compareBytesData(t, data.Get(), expectedPayload1)

	func() {
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if _, err := f.Write(payload2); err != nil {
			t.Fatal(err)
		}
	}()
	// Give it some time to handle the file content change
	time.Sleep(time.Millisecond * 500)
	compareBytesData(t, data.Get(), expectedPayload2)
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
