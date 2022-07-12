package thriftbp

import (
	"errors"
	"testing"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
)

func TestWrapErrorForServerSpan(t *testing.T) {
	var errFixed = errors.New("foo")
	for _, c := range []struct {
		label string
		err   error
		check func(t *testing.T, err error)
	}{
		{
			label: "suppress-4xx",
			err: &baseplate.Error{
				Code: thrift.Int32Ptr(400),
			},
			check: func(t *testing.T, err error) {
				if err != nil {
					t.Errorf("Expected error to be suppressed, got %#v", err)
				}
			},
		},
		{
			label: "wrap-5xx",
			err: &baseplate.Error{
				Code: thrift.Int32Ptr(500),
			},
			check: func(t *testing.T, err error) {
				if err == nil {
					t.Fatal("Expected non-nil error, got nil")
				}
				if !errors.As(err, new(wrappedBaseplateError)) {
					t.Errorf("Expect baseplate.Error to be wrapped, got %#v", err)
				}
			},
		},
		{
			label: "intact",
			err:   errFixed,
			check: func(t *testing.T, err error) {
				if err == nil {
					t.Fatal("Expected non-nil error, got nil")
				}
				if !errors.Is(err, errFixed) {
					t.Errorf("Expect %#v, got %#v", errFixed, err)
				}
			},
		},
		{
			label: "nil",
			err:   nil,
			check: func(t *testing.T, err error) {
				if err != nil {
					t.Errorf("Expected nil error, got %#v", err)
				}
			},
		},
	} {
		t.Run(c.label, func(t *testing.T) {
			c.check(t, wrapErrorForServerSpan(c.err, IDLExceptionSuppressor))
		})
	}
}
