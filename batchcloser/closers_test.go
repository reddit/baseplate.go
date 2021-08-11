package batchcloser_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/reddit/baseplate.go/batchcloser"
)

type closeRecorder struct {
	closed bool
	err    error
}

func (c *closeRecorder) Close() error {
	c.closed = true
	return c.err
}

func TestWrap(t *testing.T) {
	t.Parallel()

	t.Run(
		"no-error",
		func(t *testing.T) {
			recorder := &closeRecorder{}

			closer := batchcloser.Wrap(recorder.Close)
			if err := closer.Close(); err != nil {
				t.Fatal(err)
			}

			if !recorder.closed {
				t.Error("recorder was not closed")
			}
		},
	)

	t.Run(
		"error",
		func(t *testing.T) {
			recorder := &closeRecorder{
				err: errors.New("test error"),
			}

			closer := batchcloser.Wrap(recorder.Close)
			if err := closer.Close(); !errors.Is(err, recorder.err) {
				t.Errorf("error mismatch, expected %v, got %v", recorder.err, err)
			}

			if !recorder.closed {
				t.Error("recorder was not closed")
			}
		},
	)
}

func TestWrapCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	closer := batchcloser.WrapCancel(cancel)

	if err := closer.Close(); err != nil {
		t.Fatal(err)
	}

	if err := ctx.Err(); !errors.Is(err, context.Canceled) {
		t.Errorf("unexpected error type %v, expected %v", err, context.Canceled)
	}
}

func TestBatchCloser(t *testing.T) {
	t.Parallel()

	t.Run(
		"base",
		func(t *testing.T) {
			first := &closeRecorder{}
			second := &closeRecorder{}
			bc := batchcloser.New(first, second)
			if err := bc.Close(); err != nil {
				t.Fatal(err)
			}
			if !first.closed {
				t.Errorf("first closer was not closed.")
			}
			if !second.closed {
				t.Errorf("second closer was not closed.")
			}
		},
	)

	t.Run(
		"mixed-errors",
		func(t *testing.T) {
			first := &closeRecorder{}
			second := &closeRecorder{err: errors.New("test error")}
			bc := batchcloser.New(first, second)
			err := bc.Close()
			if !errors.Is(err, second.err) {
				t.Errorf("error from second closer was not returned.")
			}
			if !first.closed {
				t.Errorf("first closer was not closed.")
			}
			if !second.closed {
				t.Errorf("second closer was not closed.")
			}
		},
	)

	cases := []struct {
		name       string
		numClosers int
	}{
		{
			name: "zero",
		},
		{
			name:       "one",
			numClosers: 1,
		},
		{
			name:       "two",
			numClosers: 2,
		},
	}

	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				// We have to use a list of []io.Closers to use them with the `...`
				// syntax when passing to Add.
				initial := make([]io.Closer, 0, c.numClosers)
				for i := 0; i < c.numClosers; i++ {
					initial = append(initial, &closeRecorder{})
				}
				bc := batchcloser.New(initial...)
				recorder := &closeRecorder{}
				bc.Add(recorder)
				if err := bc.Close(); err != nil {
					t.Fatal(err)
				}
				for i, c := range initial {
					if closer, ok := c.(*closeRecorder); !ok {
						t.Errorf("closer %d is not a *closeRecorder", i)
					} else if !closer.closed {
						t.Errorf("closer %d was not closed", i)
					}
				}
				if !recorder.closed {
					t.Errorf("additional closer was not closed.")
				}
			},
		)

		t.Run(
			c.name+"/errors",
			func(t *testing.T) {
				// We have to use a list of []io.Closers to use them with the `...`
				// syntax when passing to Add.
				initial := make([]io.Closer, 0, c.numClosers)
				for i := 0; i < c.numClosers; i++ {
					initial = append(initial, &closeRecorder{
						err: errors.New("test error"),
					})
				}
				bc := batchcloser.New(initial...)
				recorder := &closeRecorder{
					err: errors.New("test error"),
				}
				bc.Add(recorder)

				err := bc.Close()
				if err == nil {
					t.Fatal("expected an error, got nil")
				}

				for i, c := range initial {
					if closer, ok := c.(*closeRecorder); !ok {
						t.Errorf("closer %d is not a *closeRecorder", i)
					} else {
						if !closer.closed {
							t.Errorf("closer %d was not closed", i)
						}
						if !errors.Is(err, closer.err) {
							t.Errorf("closer %d's error was not represented in the returned error", i)
						}
					}
				}
				if !recorder.closed {
					t.Errorf("additional closer was not closed.")
				}
			},
		)
	}
}
