package limitopen_test

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/reddit/baseplate.go/internal/limitopen"
)

func setup(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "file")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Fatalf("Failed to close file %q for writing: %v", path, err)
		}
	}()
	if _, err := io.Copy(f, strings.NewReader(content)); err != nil {
		t.Fatalf("Failed to write to file %q: %v", path, err)
	}
	return path
}

func TestOpen(t *testing.T) {
	const (
		content = "Hello, world!"
		max     = int64(len(content))
	)

	t.Run("read", func(t *testing.T) {
		path := setup(t, content)
		r, size, err := limitopen.Open(path)
		if err != nil {
			t.Fatalf("limitopen.Open returned error on %q: %v", path, err)
		}
		t.Cleanup(func() {
			r.Close()
		})
		if size != max {
			t.Errorf("Expected size to be %d, got %d", max, size)
		}
		var sb strings.Builder
		if _, err := io.Copy(&sb, r); err != nil {
			t.Fatalf("Failed to read from file %q: %v", path, err)
		}
		if read := sb.String(); read != content {
			t.Errorf("Expected to read %q, got %q", content, read)
		}
	})

	t.Run("close", func(t *testing.T) {
		path := setup(t, content)
		r, _, err := limitopen.Open(path)
		if err != nil {
			t.Fatalf("limitopen.Open returned error on %q: %v", path, err)
		}

		if err := r.Close(); err != nil {
			t.Fatalf("Close on %q returned error: %v", path, err)
		}
		buf := make([]byte, 1)
		if _, err := r.Read(buf); err == nil {
			t.Error("Expected error from Read after Close is called, got nothing")
		}
	})
}

func TestOpenDevZero(t *testing.T) {
	// It's hard to actually construct a file that the content you read is
	// different from the size reported by the os for test,
	// so we just use /dev/zero here.
	// At the time of writing linux kernel (5.10) is reporting 0 for the size of
	// /dev/zero. This behavior might change in the future and break this test.
	// Also only run this test on linux to avoid unexpected behaviors.
	if runtime.GOOS != `linux` {
		t.Skipf(
			"This test can only be run on Linux, skipping on %s/%s",
			runtime.GOOS,
			runtime.GOARCH,
		)
	}

	const (
		path            = "/dev/zero"
		expectedSize    = 0
		expectedContent = ""
	)
	r, size, err := limitopen.Open(path)
	if err != nil {
		t.Fatalf("limitopen.Open returned error on %q: %v", path, err)
	}
	t.Cleanup(func() {
		r.Close()
	})
	if size != expectedSize {
		t.Errorf("Expected size to be %d, got %d", expectedSize, size)
	}
	var sb strings.Builder
	if _, err := io.Copy(&sb, r); err != nil {
		t.Fatalf("Failed to read from file %q: %v", path, err)
	}
	if read := sb.String(); read != expectedContent {
		t.Errorf("Expected to read %q, got %q", expectedContent, read)
	}
}
