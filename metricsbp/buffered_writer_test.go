package metricsbp

import (
	"bytes"
	"strings"
	"testing"
)

const (
	// This message has the size of 7
	msg     = "foobar\n"
	msgSize = 7

	// We always write it 3 times
	target = msg + msg + msg
	n      = 3
)

type testWriter struct {
	bytes.Buffer

	tb              testing.TB
	expectedMaxSize int
}

func (tw *testWriter) Write(p []byte) (n int, err error) {
	tw.tb.Helper()

	content := string(p)
	tw.tb.Logf("Got write: %q", content)
	if len(p) > tw.expectedMaxSize {
		tw.tb.Errorf("Expected a single write to be no more than %d, got %q", tw.expectedMaxSize, content)
	}
	if !strings.HasSuffix(content, msg) {
		tw.tb.Errorf("Writes doesn't happen on message boundaries: %q", content)
	}
	return tw.Buffer.Write(p)
}

func (tw *testWriter) endCheck() {
	tw.tb.Helper()

	content := tw.Buffer.String()
	tw.tb.Logf("End result: %q", content)
	if content != target {
		tw.tb.Errorf("Expected content %q, got %q", target, content)
	}
}

func TestBufferedWriter(t *testing.T) {
	for _, _c := range []struct {
		label           string
		bufSize         int
		expectedMaxSize int
	}{
		{
			label:           "1",
			bufSize:         1,
			expectedMaxSize: msgSize,
		},
		{
			label:           "0",
			bufSize:         0,
			expectedMaxSize: msgSize,
		},
		{
			label:           "-1",
			bufSize:         -1,
			expectedMaxSize: msgSize,
		},
		{
			label:           "*1",
			bufSize:         msgSize,
			expectedMaxSize: msgSize,
		},
		{
			label:           "*2",
			bufSize:         msgSize * 2,
			expectedMaxSize: msgSize * 2,
		},
		{
			label:           "*2+1",
			bufSize:         msgSize*2 + 1,
			expectedMaxSize: msgSize*2 + 1,
		},
		{
			label:           "*n",
			bufSize:         msgSize * n,
			expectedMaxSize: msgSize * n,
		},
		{
			label:           "*(n+1)",
			bufSize:         msgSize * (n + 1),
			expectedMaxSize: msgSize * (n + 1),
		},
	} {
		c := _c
		t.Run(c.label, func(t *testing.T) {
			t.Parallel()
			writer := &testWriter{
				tb:              t,
				expectedMaxSize: c.expectedMaxSize,
			}
			writer.tb = t
			bufWriter := newBufferedWriter(writer, c.bufSize)
			for i := 0; i < n; i++ {
				if written, err := bufWriter.Write([]byte(msg)); err != nil {
					t.Fatalf("Failed to write #%d: %d, %v", i, written, err)
				} else if written != msgSize {
					t.Errorf("Expected written %d bytes, got %d", msgSize, written)
				}
			}
			if err := bufWriter.Flush(); err != nil {
				t.Fatalf("Failed to flush: %v", err)
			}
			writer.endCheck()
		})
	}
}
