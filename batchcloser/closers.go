package batchcloser

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/reddit/baseplate.go/errorsbp"
)

// CloseError is used to wrap the errors returned by the inner closers within a
// BatchCloser.
//
// It can be used to inspect both the error that was returned and the io.Closer
// that caused it.
type CloseError struct {
	Cause  error
	Closer io.Closer
}

// Error implements the interface for error.
func (err CloseError) Error() string {
	return fmt.Sprintf("batchcloser: error closing closer %#v : %s", err.Closer, err.Cause.Error())
}

// Unwrap implements helper interface for errors.Is.
func (err CloseError) Unwrap() error {
	return err.Cause
}

// As implements helper interface for errors.As.
func (err CloseError) As(v interface{}) bool {
	if target, ok := v.(*CloseError); ok {
		*target = err
		return true
	}
	if target, ok := v.(**CloseError); ok {
		*target = &err
		return true
	}
	if errors.As(err.Cause, v) {
		return true
	}
	return false
}

type simpleCloser struct {
	close func() error
}

func (c simpleCloser) Close() error {
	return c.close()
}

// Wrap can be used wrap close functions in an io.Closer.
func Wrap(close func() error) io.Closer {
	return simpleCloser{close: close}
}

// WrapCancel can be used to wrap a context.CancelFunc in an io.Closer.
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
// batching any errors into a errorsbp.BatchError.
func (bc *BatchCloser) Close() error {
	var errs errorsbp.BatchError
	for _, closer := range bc.closers {
		if err := closer.Close(); err != nil {
			errs.Add(CloseError{
				Cause:  err,
				Closer: closer,
			})
		}
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
	_ error     = CloseError{}
	_ error     = (*CloseError)(nil)
	_ io.Closer = simpleCloser{}
	_ io.Closer = (*simpleCloser)(nil)
	_ io.Closer = (*BatchCloser)(nil)
)
