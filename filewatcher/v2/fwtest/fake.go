package fwtest

import (
	"io"
	"io/fs"
	"sync/atomic"

	"github.com/reddit/baseplate.go/filewatcher/v2"
)

// FakeFileWatcher is an implementation of FileWatcher that does not actually
// read from a file, it simply returns the data given to it when it was
// initialized with NewFakeFilewatcher. It provides an additional Update method
// that allows you to update this data after it has been created.
type FakeFileWatcher[T any] struct {
	data   atomic.Value // type: T
	parser filewatcher.Parser[T]
}

// NewFakeFilewatcher returns a pointer to a new FakeFileWatcher object
// initialized with the given io.Reader and Parser.
func NewFakeFilewatcher[T any](r io.Reader, parser filewatcher.Parser[T]) (*FakeFileWatcher[T], error) {
	fw := &FakeFileWatcher[T]{parser: parser}
	if err := fw.Update(r); err != nil {
		return nil, err
	}
	return fw, nil
}

// Update updates the data of the FakeFileWatcher using the given io.Reader and
// the Parser used to initialize the FileWatcher.
//
// This method is not threadsafe.
func (fw *FakeFileWatcher[T]) Update(r io.Reader) error {
	data, err := fw.parser(r)
	if err != nil {
		return err
	}
	fw.data.Store(data)
	return nil
}

// Get returns the parsed data.
func (fw *FakeFileWatcher[T]) Get() T {
	return fw.data.Load().(T)
}

// Close is a no-op.
func (fw *FakeFileWatcher[T]) Close() error {
	return nil
}

// FakeDirWatcher is an implementation of FileWatcher for testing with watching
// directories.
type FakeDirWatcher[T any] struct {
	data   atomic.Value // type: T
	parser filewatcher.DirParser[T]
}

// NewFakeDirWatcher creates a FakeDirWatcher with the initial data and the
// given DirParser.
//
// It provides Update function to update the data after it's been created.
func NewFakeDirWatcher[T any](dir fs.FS, parser filewatcher.DirParser[T]) (*FakeDirWatcher[T], error) {
	dw := &FakeDirWatcher[T]{parser: parser}
	if err := dw.Update(dir); err != nil {
		return nil, err
	}
	return dw, nil
}

// Update updates the data stored in this FakeDirWatcher.
func (dw *FakeDirWatcher[T]) Update(dir fs.FS) error {
	data, err := dw.parser(dir)
	if err != nil {
		return err
	}
	dw.data.Store(data)
	return nil
}

// Get implements FileWatcher by returning the last updated data.
func (dw *FakeDirWatcher[T]) Get() T {
	return dw.data.Load().(T)
}

// Close is a no-op.
func (dw *FakeDirWatcher[T]) Close() error {
	return nil
}

var (
	_ filewatcher.FileWatcher[any] = (*FakeFileWatcher[any])(nil)
	_ filewatcher.FileWatcher[any] = (*FakeDirWatcher[any])(nil)
)
