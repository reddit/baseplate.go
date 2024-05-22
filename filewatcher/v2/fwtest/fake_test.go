package fwtest_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/reddit/baseplate.go/filewatcher/v2/fwtest"
)

func TestFakeFileWatcher(t *testing.T) {
	t.Parallel()

	const (
		foo = "foo"
		bar = "bar"
	)

	r := strings.NewReader(foo)
	fw, err := fwtest.NewFakeFilewatcher(r, func(r io.Reader) (string, error) {
		var buf bytes.Buffer
		_, err := io.Copy(&buf, r)
		if err != nil {
			return "", err
		}
		return buf.String(), nil
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Run(
		"get",
		func(t *testing.T) {
			data := fw.Get()
			if strings.Compare(data, foo) != 0 {
				t.Fatalf("%q does not match %q", data, foo)
			}
		},
	)

	t.Run(
		"update",
		func(t *testing.T) {
			if err := fw.Update(strings.NewReader(bar)); err != nil {
				t.Fatal(err)
			}

			data := fw.Get()
			if strings.Compare(data, bar) != 0 {
				t.Fatalf("%q does not match %q", data, foo)
			}
		},
	)

	t.Run(
		"errors",
		func(t *testing.T) {
			t.Run(
				"NewFakeFilewatcher",
				func(t *testing.T) {
					if _, err := fwtest.NewFakeFilewatcher(r, func(r io.Reader) (string, error) {
						return "", errors.New("test")
					}); err == nil {
						t.Fatal("expected an error, got nil")
					}
				},
			)

			t.Run(
				"update",
				func(t *testing.T) {
					fw, err := fwtest.NewFakeFilewatcher(r, func(r io.Reader) (string, error) {
						var buf bytes.Buffer
						_, err := io.Copy(&buf, r)
						if err != nil {
							return "", err
						}
						data := buf.String()
						if strings.Compare(data, bar) == 0 {
							return "", errors.New("test")
						}
						return data, nil
					})
					if err != nil {
						t.Fatal(err)
					}

					if err := fw.Update(strings.NewReader(bar)); err == nil {
						t.Fatal("expected an error, got nil")
					}
				},
			)
		},
	)
}
