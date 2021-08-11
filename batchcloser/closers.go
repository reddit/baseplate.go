package batchcloser

import (
	"context"
	"fmt"
	"io"

	"github.com/reddit/baseplate.go/errorsbp"
)

type simpleCloser struct {
	closeFunc func() error
}

func (c simpleCloser) Close() error {
	return c.closeFunc()
}

// Wrap can be used to wrap close functions into an io.Closer.
func Wrap(closeFunc func() error) io.Closer {
	return simpleCloser{
		closeFunc: closeFunc,
	}
}

// WrapCancel can be used to wrap a context.CancelFunc into an io.Closer.
func WrapCancel(cancel context.CancelFunc) io.Closer {
	return Wrap(func() error {
		cancel()
		return nil
	})
}

// New returns a pointer to a new BatchCloser initialized with the given closers.
func New(closers ...io.Closer) *BatchCloser {
	bc := &BatchCloser{}
	bc.Add(closers...)
	return bc
}

// BatchCloser is a collection of io.Closer objects that are all closed when
// BatchCloser.Close is called.
type BatchCloser struct {
	closers []io.Closer
}

// Close implements io.Closer and closes all of it's internal io.Closer objects,
// batching any errors into an errorsbp.Batch.
func (bc *BatchCloser) Close() error {
	var errs errorsbp.Batch
	for _, closer := range bc.closers {
		errs.AddPrefix(fmt.Sprintf("%#v", closer), closer.Close())
	}
	return errs.Compile()
}

// Add adds the given io.Closer objects to the BatchCloser.
//
// This is not safe to be called concurrently.
func (bc *BatchCloser) Add(closers ...io.Closer) {
	bc.closers = append(bc.closers, closers...)
}

var (
	_ io.Closer = simpleCloser{}
	_ io.Closer = (*simpleCloser)(nil)
	_ io.Closer = (*BatchCloser)(nil)
)
