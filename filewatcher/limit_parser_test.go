package filewatcher

import (
	"io"
	"io/ioutil"
	"strings"
	"testing"
)

func TestLimitParser(t *testing.T) {
	const (
		limit    = 5
		origin   = "Hello, world!"
		expected = "Hello"
	)

	parser := func(data io.Reader) (interface{}, error) {
		buf, err := ioutil.ReadAll(data)
		if err != nil {
			t.Error(err)
			return nil, err
		}
		if string(buf) != expected {
			t.Errorf("Data expected %q, got %q", expected, buf)
		}
		return nil, nil
	}
	limitParser(parser, limit)(strings.NewReader(origin))
}
