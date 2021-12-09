package errorsbp

import (
	"errors"
	"testing"
)

func TestBatchSize(t *testing.T) {
	for _, c := range []struct {
		label string
		err   error
		want  int
	}{
		{
			label: "nil",
			err:   nil,
			want:  0,
		},
		{
			label: "non-batch",
			err:   errors.New("foo"),
			want:  1,
		},
		{
			label: "batch-0",
			err:   new(Batch),
			want:  0,
		},
		{
			label: "batch-1",
			err: Batch{
				errors: []error{errors.New("bar")},
			},
			want: 1,
		},
		{
			label: "batch-2",
			err: &Batch{
				errors: []error{errors.New("foo"), errors.New("bar")},
			},
			want: 2,
		},
	} {
		t.Run(c.label, func(t *testing.T) {
			if got := BatchSize(c.err); got != c.want {
				t.Errorf("Expected BatchSize(%v) to return %d, got %d", c.err, c.want, got)
			}
		})
	}
}
