package iobp_test

import (
	"bytes"
	"fmt"
	"io"
	"sync"
	"testing"
	"testing/quick"

	"github.com/reddit/baseplate.go/iobp"
)

func TestCountingSink(t *testing.T) {
	f := func(s uint16) bool {
		buf := make([]byte, s)
		var sink iobp.CountingSink
		if _, err := io.Copy(&sink, bytes.NewReader(buf)); err != nil {
			t.Errorf("Failed to copy into CountingSink: %v", err)
		}
		if size := sink.Size(); size != int64(s) {
			t.Errorf("Expected size %d, got %d", s, size)
		}
		return !t.Failed()
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func BenchmarkCountingSink(b *testing.B) {
	bufPool := sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer)
		},
	}
	cases := []struct {
		label   string
		counter func(testing.TB, io.Reader) int64
	}{
		{
			label: "pooled-buffer",
			counter: func(tb testing.TB, r io.Reader) int64 {
				buf := bufPool.Get().(*bytes.Buffer)
				buf.Reset()
				defer func() {
					bufPool.Put(buf)
				}()
				if _, err := io.Copy(buf, r); err != nil {
					tb.Error(err)
				}
				return int64(buf.Len())
			},
		},
		{
			label: "unpooled-buffer",
			counter: func(tb testing.TB, r io.Reader) int64 {
				var buf bytes.Buffer
				if _, err := io.Copy(&buf, r); err != nil {
					tb.Error(err)
				}
				return int64(buf.Len())
			},
		},
		{
			label: "CountingSink",
			counter: func(tb testing.TB, r io.Reader) int64 {
				var sink iobp.CountingSink
				if _, err := io.Copy(&sink, r); err != nil {
					tb.Error(err)
				}
				return sink.Size()
			},
		},
	}
	for _, size := range []int{10, 100, 1000, 10000} {
		b.Run(fmt.Sprintf("size-%d", size), func(b *testing.B) {
			buf := make([]byte, size)
			for _, c := range cases {
				b.Run(c.label, func(b *testing.B) {
					b.ReportAllocs()

					if actual := c.counter(b, bytes.NewReader(buf)); actual != int64(size) {
						b.Fatalf("Expected counted size to be %d, got %d", size, actual)
					}
					b.ResetTimer()

					b.RunParallel(func(pb *testing.PB) {
						for pb.Next() {
							c.counter(b, bytes.NewReader(buf))
						}
					})
				})
			}
		})
	}
}
