package filewatcher_test

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.snooguts.net/reddit/baseplate.go/filewatcher"
)

func parser(f io.Reader) (interface{}, error) {
	return ioutil.ReadAll(f)
}

func compareBytesData(t *testing.T, data interface{}, expected []byte) {
	t.Helper()

	if data == nil {
		t.Fatal("data is nil")
	}
	b, ok := (data).([]byte)
	if !ok {
		t.Errorf("data is not of type *[]byte, actual value: %v", data)
	}
	if string(b) != string(expected) {
		t.Errorf("*data expected to be %q, got %q", expected, b)
	}
}

func TestFileWatcher(t *testing.T) {
	interval := time.Millisecond
	filewatcher.InitialReadInterval = interval
	writeDelay := interval * 10
	timeout := writeDelay * 2

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
		path,
		parser,
		nil, // logger
	)
	if err != nil {
		t.Fatal(err)
	}
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
	time.Sleep(time.Millisecond * 10)
	compareBytesData(t, data.Get(), payload2)
}

func TestFileWatcherTimeout(t *testing.T) {
	interval := time.Millisecond
	filewatcher.InitialReadInterval = interval
	round := interval * 10
	timeout := round * 5

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
		path,
		parser,
		nil, // logger
	)
	if err == nil {
		t.Error("Expected context cancellation error, got nil.")
	}
	duration := time.Since(before)
	if duration.Round(round) > timeout.Round(round) {
		t.Errorf("Timeout took %v instead of %v", duration, timeout)
	}
}

func TestFileWatcherRename(t *testing.T) {
	interval := time.Millisecond
	filewatcher.InitialReadInterval = interval
	writeDelay := interval * 10
	timeout := writeDelay * 2

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
		path,
		parser,
		nil, // logger
	)
	if err != nil {
		t.Fatal(err)
	}
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
	time.Sleep(time.Millisecond * 10)
	compareBytesData(t, data.Get(), payload2)
}
