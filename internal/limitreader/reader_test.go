package limitreader_test

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/reddit/baseplate.go/internal/limitreader"
)

func TestReader(t *testing.T) {
	const (
		content = "Hello, world!"
		max     = int64(len(content))
	)

	for _, c := range []struct {
		label    string
		limit    int64
		n        int64
		expected string
		err      error
	}{
		{
			label:    "normal",
			limit:    max,
			n:        max,
			expected: content,
			err:      nil,
		},
		{
			// When n is larger than limit, it should return EOF.
			label:    "larger-n",
			limit:    max,
			n:        max + 1,
			expected: content,
			err:      io.EOF,
		},
		{
			// When limit is larger than the reader, it should return EOF.
			label:    "larger-limit",
			limit:    max + 1,
			n:        max + 1,
			expected: content,
			err:      io.EOF,
		},
		{
			// When limit is smaller than the size and n is larger than limit,
			// it should return ErrReadBeyondLimit.
			label:    "smaller-limit",
			limit:    max - 1,
			n:        max,
			expected: content[:max-1],
			err:      limitreader.ErrReadLimitExceeded,
		},
	} {
		t.Run(c.label, func(t *testing.T) {
			r := limitreader.New(strings.NewReader(content), c.limit)
			var sb strings.Builder
			_, err := io.CopyN(&sb, r, c.n)
			if s := sb.String(); s != c.expected {
				t.Errorf("Expected to read %q, got %q", c.expected, s)
			}
			if !errors.Is(err, c.err) {
				t.Errorf("Expected error %v, got %v", c.err, err)
			}
		})
	}
}
